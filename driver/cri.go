package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	defaultPodImage        = "gcr.io/google_containers/pause:3.0"
	defaultPodNamePrefix   = "pod"
	defaultSandboxConfig   = "contrib/sandbox_config.json"
	defaultContainerConfig = "contrib/container_config.json"
)

var (
	pconfigGlobal pb.PodSandboxConfig
	cconfigGlobal pb.ContainerConfig
)

// CRIDriver is an implementation of the driver interface for using k8s Container Runtime Interface.
// This uses the provided client library which abstracts using the gRPC APIs directly.
type CRIDriver struct {
	criSocketAddress string
	runtimeClient    *pb.RuntimeServiceClient
	imageClient      *pb.ImageServiceClient
	pconfig          pb.PodSandboxConfig
	cconfig          pb.ContainerConfig
}

// CRIContainer is an implementation of the container metadata needed for CRI implementation
type CRIContainer struct {
	name        string
	imageName   string
	cmdOverride string
	state       string
	process     string
	trace       bool
	podID       string
}

// NewCRIDriver creates an instance of the CRI driver
func NewCRIDriver(path string) (Driver, error) {
	if path == "" {
		return nil, fmt.Errorf("socket path unspecified")
	}

	conn, err := getGRPCConn(path, time.Duration(10*time.Second))
	if err != nil {
		return nil, err
	}

	runtimeClient := pb.NewRuntimeServiceClient(conn)
	imageClient := pb.NewImageServiceClient(conn)

	pconfig, err := loadPodSandboxConfig(defaultSandboxConfig)
	if err != nil {
		return nil, err
	}

	cconfig, err := loadContainerConfig(defaultContainerConfig)
	if err != nil {
		return nil, err
	}

	driver := &CRIDriver{
		criSocketAddress: path,
		runtimeClient:    &runtimeClient,
		imageClient:      &imageClient,
		cconfig:          cconfig,
		pconfig:          pconfig,
	}

	return driver, nil
}

func getGRPCConn(socket string, timeout time.Duration) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(socket, grpc.WithInsecure(), grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

// Name returns the name of the container
func (ctr *CRIContainer) Name() string {
	return ctr.name
}

// Detached returns whether the container is to be started in detached state
func (ctr *CRIContainer) Detached() bool {
	return true
}

// Trace returns whether the container should be traced (using any tracing supported
// by the container runtime)
func (ctr *CRIContainer) Trace() bool {
	return ctr.trace
}

// Image returns either a bundle path (used by runc, containerd) or image name (used by Docker)
// that will be used by the container runtime to know what image to run/execute
func (ctr *CRIContainer) Image() string {
	return ctr.imageName
}

// Command returns an optional command that overrides the default image
// "CMD" or "ENTRYPOINT" for the Docker and Containerd (gRPC) drivers
func (ctr *CRIContainer) Command() string {
	return ctr.cmdOverride
}

//GetPodID return pod-id associated with container.
func (ctr *CRIContainer) GetPodID() string {
	return ctr.podID
}

// Type returns a driver type to identify the driver
func (c *CRIDriver) Type() Type {
	return CRI
}

// Info returns a string with information about the container engine/runtime details
func (c *CRIDriver) Info(ctx context.Context) (string, error) {
	version, err := (*c.runtimeClient).Version(ctx, &pb.VersionRequest{})
	if err != nil {
		return "", err
	}

	info := "CRI Client driver (Version: " + version.GetVersion() + ", API Version: " + version.GetRuntimeApiVersion() + " Runtime" + version.GetRuntimeName() + version.GetRuntimeVersion() + " )"

	return info, nil
}

// Path returns the binary (or socket) path related to the runtime in use
func (c *CRIDriver) Path() string {
	return c.criSocketAddress
}

// Create will create a container instance matching the specific needs
// of a driver
func (c *CRIDriver) Create(ctx context.Context, name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	if status, err := (*c.imageClient).ImageStatus(ctx, &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: image}}); err != nil || status.Image == nil {
		if _, err := (*c.imageClient).PullImage(ctx, &pb.PullImageRequest{Image: &pb.ImageSpec{Image: image}}); err != nil {
			return nil, err
		}
	}

	if status, err := (*c.imageClient).ImageStatus(ctx, &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: defaultPodImage}}); err != nil || status.Image == nil {
		if _, err := (*c.imageClient).PullImage(ctx, &pb.PullImageRequest{Image: &pb.ImageSpec{Image: defaultPodImage}}); err != nil {
			return nil, err
		}
	}

	pconfig := pconfigGlobal
	pconfig.Metadata.Name = defaultPodNamePrefix + name

	podInfo, err := (*c.runtimeClient).RunPodSandbox(ctx, &pb.RunPodSandboxRequest{Config: &pconfig})
	if err != nil {
		return nil, err
	}

	containerObj := &CRIContainer{
		name:        name,
		imageName:   image,
		cmdOverride: cmdOverride,
		trace:       trace,
		podID:       podInfo.GetPodSandboxId(),
	}

	return containerObj, nil
}

// Clean will clean the operating environment of a specific driver
func (c CRIDriver) Clean(ctx context.Context) error {

	resp, err := (*c.runtimeClient).ListContainers(ctx, &pb.ListContainersRequest{Filter: &pb.ContainerFilter{}})
	if err != nil {
		return err
	}
	containers := resp.GetContainers()
	for _, ctr := range containers {
		podID := ctr.GetPodSandboxId()
		_, err := (*c.runtimeClient).StopContainer(ctx, &pb.StopContainerRequest{ContainerId: ctr.GetId(), Timeout: 0})
		if err != nil {
			log.Errorf("Error stopping container: %v", err)
		}
		_, err = (*c.runtimeClient).RemoveContainer(ctx, &pb.RemoveContainerRequest{ContainerId: ctr.GetId()})
		if err != nil {
			log.Errorf("Error deleting container %v", err)
		}
		_, err = (*c.runtimeClient).RemovePodSandbox(ctx, &pb.RemovePodSandboxRequest{PodSandboxId: podID})
		if err != nil {
			log.Errorf("Error deleting pod %s, %v", podID, err)
		}
	}
	log.Infof("CRI cleanup complete.")
	return nil
}

// Run will execute a container using the driver
func (c *CRIDriver) Run(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()
	cconfig := cconfigGlobal
	pconfig := pconfigGlobal
	cconfig.Metadata.Name = ctr.Name()
	pconfig.Metadata.Name = defaultPodNamePrefix + cconfig.Metadata.Name

	_, err := (*c.runtimeClient).CreateContainer(ctx, &pb.CreateContainerRequest{PodSandboxId: ctr.GetPodID(), Config: &cconfig, SandboxConfig: &pconfig})
	if err != nil {
		return "", 0, err
	}
	elapsed := time.Since(start)
	return "", elapsed, nil
}

// Stop will stop/kill a container
func (c *CRIDriver) Stop(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()
	resp, err := (*c.runtimeClient).ListContainers(ctx, &pb.ListContainersRequest{Filter: &pb.ContainerFilter{PodSandboxId: ctr.GetPodID()}})
	if err != nil {
		return "", 0, nil
	}

	containers := resp.GetContainers()
	for _, ctr := range containers {
		podID := ctr.GetPodSandboxId()
		_, err := (*c.runtimeClient).StopContainer(ctx, &pb.StopContainerRequest{ContainerId: ctr.GetId(), Timeout: 0})
		if err != nil {
			log.Errorf("Error Stoping container %v", err)
			return "", 0, nil
		}
		_, err = (*c.runtimeClient).StopPodSandbox(ctx, &pb.StopPodSandboxRequest{PodSandboxId: podID})
		if err != nil {
			log.Errorf("Error Stoping pod %v", err)
			return "", 0, nil
		}
	}
	elapsed := time.Since(start)
	return "", elapsed, nil
}

// Remove will remove a container
func (c *CRIDriver) Remove(ctx context.Context, ctr Container) (string, time.Duration, error) {

	start := time.Now()
	resp, err := (*c.runtimeClient).ListContainers(ctx, &pb.ListContainersRequest{Filter: &pb.ContainerFilter{PodSandboxId: ctr.GetPodID()}})
	if err != nil {
		return "", 0, nil
	}

	containers := resp.GetContainers()
	for _, ctr := range containers {
		podID := ctr.GetPodSandboxId()
		_, err = (*c.runtimeClient).RemoveContainer(ctx, &pb.RemoveContainerRequest{ContainerId: ctr.GetId()})
		if err != nil {
			log.Errorf("Error deleting container %v", err)
			return "", 0, nil
		}
		_, err = (*c.runtimeClient).RemovePodSandbox(ctx, &pb.RemovePodSandboxRequest{PodSandboxId: podID})
		if err != nil {
			log.Errorf("Error deleting pod %v", err)
			return "", 0, nil
		}
	}
	elapsed := time.Since(start)
	return "", elapsed, nil
}

// Pause will pause a container
// not supported in CRI API
func (c *CRIDriver) Pause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return "", 0, nil
}

// Unpause will unpause/resume a container
// not supported in CRI API
func (c *CRIDriver) Unpause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return "", 0, nil
}

// Close allows the driver to free any resources/close any
// connections
func (c *CRIDriver) Close() error {
	return nil
}

func (c *CRIDriver) PID() (int, error) {
	return 0, errors.New("not implemented")
}

func (c *CRIDriver) Wait(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return "", 0, errors.New("not implemented")
}

func (c *CRIDriver) Metrics(ctx context.Context, ctr Container) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (c *CRIDriver) ProcNames() []string {
	return []string{}
}

func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file %s not found", path)
		}
		return nil, err
	}
	return f, nil
}

func loadPodSandboxConfig(path string) (pb.PodSandboxConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return pb.PodSandboxConfig{}, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&pconfigGlobal); err != nil {
		return pb.PodSandboxConfig{}, err
	}
	return pconfigGlobal, nil
}

func loadContainerConfig(path string) (pb.ContainerConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return pb.ContainerConfig{}, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cconfigGlobal); err != nil {
		return pb.ContainerConfig{}, err
	}
	return cconfigGlobal, nil
}
