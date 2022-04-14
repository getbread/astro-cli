package airflow

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	containerTypes "github.com/astronomer/astro-cli/airflow/types"

	"github.com/astronomer/astro-cli/messages"

	clicommand "github.com/docker/cli/cli/command"
	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	log "github.com/sirupsen/logrus"
)

type DockerImage struct {
	imageName string
}

func DockerImageInit(image string) *DockerImage {
	// We use latest and keep this tag around after deployments to keep subsequent deploys quick
	return &DockerImage{imageName: image}
}

func (d *DockerImage) Build(config containerTypes.ImageBuildConfig) error {
	// Change to location of Dockerfile
	err := os.Chdir(config.Path)
	if err != nil {
		return err
	}

	dockerfile := "Dockerfile"
	imageName := imageName(d.imageName, "latest")

	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Debugf("Error setting up new Client ops %v", err)
		panic(err)
	}

	// Create a buffer
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	// Create a filereader
	dockerFileReader, err := os.Open(dockerfile)
	if err != nil {
		return err
	}

	// Read the actual Dockerfile
	readDockerFile, err := ioutil.ReadAll(dockerFileReader)
	if err != nil {
		return err
	}

	// Make a TAR header for the file
	tarHeader := &tar.Header{
		Name: dockerfile,
		Size: int64(len(readDockerFile)),
	}

	// Writes the header described for the TAR file
	err = tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}

	// Writes the dockerfile data to the TAR file
	_, err = tw.Write(readDockerFile)
	if err != nil {
		return err
	}

	dockerFileTarReader := bytes.NewReader(buf.Bytes())

	// Define the build options to use for the file
	// https://godoc.org/github.com/docker/docker/api/types#ImageBuildOptions
	buildOptions := types.ImageBuildOptions{
		Context:    dockerFileTarReader,
		Dockerfile: dockerfile,
		Remove:     true,
		Tags:       []string{imageName},
		NoCache:    config.NoCache,
	}

	// Build the actual image
	imageBuildResponse, err := cli.ImageBuild(
		ctx,
		dockerFileTarReader,
		buildOptions,
	)

	if err != nil {
		return err
	}

	// Read the STDOUT from the build process
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *DockerImage) Push(cloudDomain, token, remoteImageTag string) error {
	registry := "registry." + cloudDomain
	remoteImage := fmt.Sprintf("%s/%s", registry, imageName(d.imageName, remoteImageTag))

	err := dockerExec(nil, nil, "tag", imageName(d.imageName, "latest"), remoteImage)
	if err != nil {
		return fmt.Errorf("command 'docker tag %s %s' failed: %w", d.imageName, remoteImage, err)
	}

	// Push image to registry
	fmt.Println(messages.PushingImagePrompt)

	configFile := cliconfig.LoadDefaultConfigFile(os.Stderr)

	authConfig, err := configFile.GetAuthConfig(registry)
	// TODO: rethink how to reuse creds store
	authConfig.Password = token

	log.Debugf("Exec Push docker creds %v \n", authConfig)
	if err != nil {
		log.Debugf("Error reading credentials: %v", err)
		return fmt.Errorf("error reading credentials: %w", err)
	}

	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Debugf("Error setting up new Client ops %v", err)
		panic(err)
	}
	cli.NegotiateAPIVersion(ctx)
	buf, err := json.Marshal(authConfig)
	if err != nil {
		log.Debugf("Error negotiating api version: %v", err)
		return err
	}
	encodedAuth := base64.URLEncoding.EncodeToString(buf)
	responseBody, err := cli.ImagePush(ctx, remoteImage, types.ImagePushOptions{RegistryAuth: encodedAuth})
	if err != nil {
		log.Debugf("Error pushing image to docker: %v", err)
		return err
	}
	defer responseBody.Close()
	out := clicommand.NewOutStream(os.Stdout)
	err = jsonmessage.DisplayJSONMessagesToStream(responseBody, out, nil)
	if err != nil {
		return err
	}

	// Delete the image tags we just generated
	err = dockerExec(nil, nil, "rmi", remoteImage)
	if err != nil {
		return fmt.Errorf("command 'docker rmi %s' failed: %w", remoteImage, err)
	}
	return nil
}

func (d *DockerImage) GetImageLabels() (map[string]string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	var labels map[string]string
	err := dockerExec(stdout, stderr, "inspect", "--format", "{{ json .Config.Labels }}", imageName(d.imageName, "latest"))
	if err != nil {
		return labels, err
	}
	if execErr := stderr.String(); execErr != "" {
		return labels, fmt.Errorf("%s: %w", execErr, errGetImageLabel)
	}
	err = json.Unmarshal(stdout.Bytes(), &labels)
	if err != nil {
		return labels, err
	}
	return labels, nil
}

// Exec executes a docker command
var dockerExec = func(stdout, stderr io.Writer, args ...string) error {
	_, lookErr := exec.LookPath(Docker)
	if lookErr != nil {
		return fmt.Errorf("failed to find the docker binary: %w", lookErr)
	}

	cmd := exec.Command(Docker, args...)
	cmd.Stdin = os.Stdin
	if stdout == nil {
		cmd.Stdout = os.Stdout
	} else {
		cmd.Stdout = stdout
	}

	if stderr == nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = stderr
	}

	if cmdErr := cmd.Run(); cmdErr != nil {
		return fmt.Errorf("failed to execute cmd: %w", cmdErr)
	}

	return nil
}
