package airflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/archive"
	"github.com/moby/term"
	"io"
	"os"
	"os/exec"
	"path/filepath"

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

	// Create a docker client
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Debugf("Error setting up new Client ops %v", err)
		panic(err)
	}

	// Create a tar of the Docker build context
	filePath := filepath.Join(config.Path)
	dockerBuildContext, err := archive.TarWithOptions(filePath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	// Define the build options to use for the file
	buildOptions := types.ImageBuildOptions{
		Context:    dockerBuildContext,
		Dockerfile: dockerfile,
		Remove:     true,
		Tags:       []string{imageName},
		NoCache:    config.NoCache,
	}

	// Build the actual image
	imageBuildResponse, err := dockerClient.ImageBuild(
		ctx,
		dockerBuildContext,
		buildOptions,
	)
	if err != nil {
		return err
	}

	// Read the STDOUT from the build process
	defer imageBuildResponse.Body.Close()

	termFd, isTerm := term.GetFdInfo(os.Stdout)
	err = jsonmessage.DisplayJSONMessagesStream(imageBuildResponse.Body, os.Stdout, termFd, isTerm, nil)
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
	// Create a docker client
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Debugf("Error setting up new Client ops %v", err)
		panic(err)
	}

	var labels map[string]string

	// Use an image list filter to get the ID of the image
	filter := filters.NewArgs()
	fullImageName := imageName(d.imageName, "latest")
	filter.Add("reference", fullImageName)
	imageList, err := dockerClient.ImageList(ctx, types.ImageListOptions{Filters: filter})
	if err != nil {
		return labels, err
	}

	// Return an error if the image isn't found
	if len(imageList) == 0 {
		return labels, fmt.Errorf("docker image %s not found: %v", fullImageName, errGetImageLabel)
	}

	// Get the image's labels by using the image ID
	imageInspect, _, err := dockerClient.ImageInspectWithRaw(ctx, imageList[0].ID)
	if err != nil {
		return labels, err
	}

	labels = imageInspect.Config.Labels
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
