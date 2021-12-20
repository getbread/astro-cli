package airflow

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/astronomer/astro-cli/config"
	"github.com/astronomer/astro-cli/messages"

	"github.com/compose-spec/compose-go/loader"
	cTypes "github.com/compose-spec/compose-go/types"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	dockerStateUp = "Up"

	projectStopTimeout = 5

	// Docker is the docker command.
	Docker = "docker"
)

type DockerCompose struct {
	airflowHome    string
	projectName    string
	envFile        string
	composeService api.Service
}

func DockerComposeInit(airflowHome, envFile string) (*DockerCompose, error) {
	// Get project name from config
	projectName, err := projectNameUnique()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving working directory")
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing docker client")
	}
	composeService := compose.NewComposeService(dockerClient, &configfile.ConfigFile{})

	return &DockerCompose{
		airflowHome:    airflowHome,
		projectName:    projectName,
		envFile:        envFile,
		composeService: composeService,
	}, nil
}

func (d *DockerCompose) Start(dockerfile string) error {
	project, err := createProject(d.projectName, d.airflowHome, d.envFile)
	if err != nil {
		return err
	}

	// Get project containers
	psInfo, err := d.composeService.Ps(context.TODO(), d.projectName, api.PsOptions{All: true})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerStatusCheck)
	}

	if len(psInfo) > 0 {
		// Ensure project is not already running
		for idx := range psInfo {
			info := psInfo[idx]
			if checkServiceState(info.State, dockerStateUp) {
				return errors.New("cannot start, project already running")
			}
		}
	}

	// Build this project image
	imageBuilder := DockerImageInit(d.projectName)
	err = imageBuilder.Build(".")
	if err != nil {
		return err
	}

	// Start up our project
	err = d.composeService.Up(context.TODO(), project, api.UpOptions{})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerRecreate)
	}

	parts := strings.Split(config.CFG.WebserverPort.GetString(), ":")
	fmt.Printf(messages.ContainerLinkWebserver+"\n", parts[len(parts)-1])
	fmt.Printf(messages.ContainerLinkPostgres+"\n", config.CFG.PostgresPort.GetString())
	fmt.Printf(messages.ContainerUserPassword + "\n")

	return nil
}

func (d *DockerCompose) Kill() error {
	// Shut down our project
	err := d.composeService.Down(context.TODO(), d.projectName, api.DownOptions{Volumes: true, RemoveOrphans: true})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerStop)
	}

	return nil
}

func (d *DockerCompose) Logs(follow bool, containerNames ...string) error {
	psInfo, err := d.composeService.Ps(context.TODO(), d.projectName, api.PsOptions{All: true})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerStatusCheck)
	}

	if len(psInfo) == 0 {
		return errors.New("cannot view logs, project not running")
	}

	logger := &ComposeLogger{logger: logrus.New()}
	err = d.composeService.Logs(context.TODO(), d.projectName, logger, api.LogOptions{Services: containerNames, Follow: follow})
	if err != nil {
		return err
	}

	return nil
}

func (d *DockerCompose) Stop() error {
	project, err := createProject(d.projectName, d.airflowHome, d.envFile)
	if err != nil {
		return err
	}
	// Pause our project
	stopTimeout := time.Duration(projectStopTimeout)
	err = d.composeService.Stop(context.TODO(), project, api.StopOptions{Timeout: &stopTimeout})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerPause)
	}

	return nil
}

func (d *DockerCompose) PS() error {
	// List project containers
	psInfo, err := d.composeService.Ps(context.TODO(), d.projectName, api.PsOptions{All: true})
	if err != nil {
		return errors.Wrap(err, messages.ErrContainerStatusCheck)
	}

	// Columns for table
	infoColumns := []string{"Name", "State", "Ports"}

	// Create a new tabwriter
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 0, 8, 2, '\t', tabwriter.AlignRight) // nolint:gomnd

	// Append data to table
	// Fix this
	fmt.Fprintln(tw, strings.Join(infoColumns, "\t"))
	for idx := range psInfo {
		info := psInfo[idx]
		ports := []string{}
		for _, port := range info.Publishers {
			ports = append(ports, strconv.Itoa(port.PublishedPort))
		}
		data := []string{info.Name, info.State, strings.Join(ports, ",")}
		fmt.Fprintln(tw, strings.Join(data, "\t"))
	}

	// Flush to stdout
	return tw.Flush()
}

func (d *DockerCompose) Run(args []string, user string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	execConfig := &types.ExecConfig{
		AttachStdout: true,
		Cmd:          args,
	}
	if user != "" {
		execConfig.User = user
	}

	fmt.Printf("Running: %s\n", strings.Join(args, " "))
	containerID, err := d.getWebServerContainerID()
	if err != nil {
		return err
	}

	response, err := cli.ContainerExecCreate(context.Background(), containerID, *execConfig)
	if err != nil {
		return errors.New("airflow is not running, Start it with 'astro airflow start'")
	}

	execID := response.ID
	if execID == "" {
		return errors.New("exec ID is empty")
	}

	execStartCheck := types.ExecStartCheck{
		Detach: execConfig.Detach,
	}

	resp, _ := cli.ContainerExecAttach(context.Background(), execID, execStartCheck)

	return execPipe(resp, os.Stdin, os.Stdout, os.Stderr)
}

// imageBuild builds the airflow project
func (d *DockerCompose) Build(imageName string) error {
	// Change to location of Dockerfile
	err := os.Chdir(d.airflowHome)
	if err != nil {
		return err
	}

	// Build image
	err = dockerExec("build", "-t", imageName, ".")
	if err != nil {
		return errors.Wrapf(err, "command 'docker build -t %s failed", imageName)
	}

	return nil
}

// ExecCommand executes a command on webserver container, and sends the response as string, this can be clubbed with Run()
func (d *DockerCompose) ExecCommand(containerID, command string) string {
	cmd := exec.Command("docker", "exec", "-it", containerID, "bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		_ = errors.Wrapf(err, "error encountered")
	}

	stringOut := string(out)
	return stringOut
}

func (d *DockerCompose) GetContainerID(containerName string) (string, error) {
	psInfo, err := d.composeService.Ps(context.TODO(), d.projectName, api.PsOptions{All: true})
	if err != nil {
		return "", errors.Wrap(err, messages.ErrContainerStatusCheck)
	}

	for idx := range psInfo {
		info := psInfo[idx]
		if strings.Contains(info.Name, d.projectName) &&
			strings.Contains(info.Name, containerName) {
			return info.ID, nil
		}
	}
	return "", err
}

// getWebServerContainerID return webserver container id
func (d *DockerCompose) getWebServerContainerID() (string, error) {
	return d.GetContainerID("webserver")
}

// createProject creates project with yaml config as context
func createProject(projectName, airflowHome, envFile string) (*cTypes.Project, error) {
	// Generate the docker-compose yaml
	yaml, err := generateConfig(projectName, airflowHome, envFile, DockerEngine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create project")
	}

	if err != nil {
		return nil, err
	}

	var configs []cTypes.ConfigFile
	composeConfig := cTypes.ConfigFile{
		Content:  []byte(yaml),
		Filename: "docker-compose.yml",
	}
	configs = append(configs, composeConfig)

	composeFile := "docker-compose.override.yml"
	composeBytes, err := ioutil.ReadFile(composeFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "Failed to open the compose file: %s", composeFile)
	}
	if err == nil {
		overrideConfig := cTypes.ConfigFile{Content: composeBytes, Filename: composeFile}
		configs = append(configs, overrideConfig)
	}

	loaderOption := func(opts *loader.Options) {
		opts.Name = projectName
	}

	project, err := loader.Load(cTypes.ConfigDetails{
		ConfigFiles: configs,
		WorkingDir:  airflowHome,
		Environment: map[string]string{},
	}, loaderOption)

	return project, err
}

func checkServiceState(serviceState, expectedState string) bool {
	scrubbedState := strings.Split(serviceState, " ")[0]
	return scrubbedState == expectedState
}

// Exec executes a docker command
func dockerExec(args ...string) error {
	_, lookErr := exec.LookPath(Docker)
	if lookErr != nil {
		return errors.Wrap(lookErr, "failed to find the docker binary")
	}

	cmd := exec.Command(Docker, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if cmdErr := cmd.Run(); cmdErr != nil {
		return errors.Wrapf(cmdErr, "failed to execute cmd")
	}

	return nil
}

// execPipe does pipe stream into stdout/stdin and stderr
// so now we can pipe out during exec'ing any commands inside container
func execPipe(resp types.HijackedResponse, inStream io.Reader, outStream, errorStream io.Writer) error {
	var err error
	receiveStdout := make(chan error, 1)
	if outStream != nil || errorStream != nil {
		go func() {
			// always do this because we are never tty
			_, err = stdcopy.StdCopy(outStream, errorStream, resp.Reader)
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inStream != nil {
			_, err := io.Copy(resp.Conn, inStream)
			if err != nil {
				fmt.Println("Error copying input stream: ", err.Error())
			}
		}

		err := resp.CloseWrite()
		if err != nil {
			fmt.Println("Error closing response body: ", err.Error())
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			return err
		}
	case <-stdinDone:
		if outStream != nil || errorStream != nil {
			if err := <-receiveStdout; err != nil {
				return err
			}
		}
	}

	return nil
}

type ComposeLogger struct {
	logger *logrus.Logger
}

func (l *ComposeLogger) Log(service, container, message string) {
	l.logger.Infof("%s | %s", service, message)
}

func (l *ComposeLogger) Status(container, msg string) {
	l.logger.Infof("%s | %s", container, msg)
}

func (l *ComposeLogger) Register(container string) {
}
