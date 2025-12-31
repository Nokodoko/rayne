package pl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

var IMAGE string = "gcr.io/datadoghq/synthetics-private-location-worker:latest"

func runner(command *exec.Cmd, errMsg string) (int, any, error) {
	output, cmdErr := command.Output()
	if cmdErr != nil {
		imageErr := NewImageError(cmdErr, command.ProcessState.ExitCode())
		log.Fatalf("%s: %v", errMsg, imageErr)
		return http.StatusInternalServerError, nil, cmdErr
	}
	return http.StatusOK, output, nil
}

func runnerNoOutput(command *exec.Cmd, errMsg string) (int, any, error) {
	_, cmdErr := command.Output()
	if cmdErr != nil {
		imageErr := NewImageError(cmdErr, command.ProcessState.ExitCode())
		log.Fatalf("%s: %v", errMsg, imageErr)
		return http.StatusInternalServerError, nil, cmdErr
	}
	return http.StatusInternalServerError, nil, cmdErr
}

func pushPull() (int, any, error) {
	status, _, err := ImageRemover()
	if err != nil {
		status = http.StatusInternalServerError
		log.Println(err)
		return status, nil, err
	}

	pullstatus, _, podmanErr := podmanPullImage()
	if podmanErr != nil {
		pullstatus = http.StatusInternalServerError
		log.Println(podmanErr)
		return pullstatus, nil, podmanErr
	}

	return http.StatusOK, nil, nil
}

func podmanPullImage() (int, any, error) {
	fmt.Println("---\n PULLING PRIVATE LOCATION IMAGE")
	// cmd := exec.Command("podman", "run", "-d", "--restart", "always", "--name", "eauth", "-v", "/etc/datadog/worker-config-e_auth-1dfda7758e6eb02738989ddc03348e62.json:/etc/datadog/synthetics-check-runner.json", "-v", "/etc/datadog/certs:/etc/datadog/certs", "gcr.io/datadoghq/synthetics-private-location-worker:latest")
	cmd := exec.Command("podman", "run", "--restart", "always", "--name", "eauth", "--replace",
		"-v", "/etc/datadog/worker-config-e_auth-1dfda7758e6eb02738989ddc03348e62.json:/etc/datadog/synthetics-check-runner.json",
		"-v", "/etc/datadog/certs:/etc/datadog/certs",
		"gcr.io/datadoghq/synthetics-private-location-worker:latest")
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Error pulling image:", err)
		return http.StatusInternalServerError, nil, err
	}
	imageproof := exec.Command("podman", "ps")
	output, err := imageproof.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	log.Println(string(output))
	fmt.Println("---\n Updated IMAGE")
	return http.StatusOK, output, nil
}

func ImageRemover() (int, any, error) {
	cmd := exec.Command("podman", "image", "rm", "--force", "gcr.io/datadoghq/synthetics-private-location-worker")
	fmt.Println("--- Removing Stale Image")
	_, _, err := runnerNoOutput(cmd, "Error Removing Stale Image(s): ")
	if err != nil {
		log.Printf("Error removing Image")
		return http.StatusInternalServerError, nil, err
	}

	imageproof := exec.Command("podman", "images")
	output, _, err := runner(imageproof, "Cannot run podman images")
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, output, nil
}

// NOTE: dead code (managing containers with systemd)
func containerStopper() (int, any, error) {
	cmd := exec.Command("podman", "stop", "eauth")
	_, _, err := runnerNoOutput(cmd, "Error stopping Container: ")
	if err != nil {
		return http.StatusNoContent, nil, err
	}
	fmt.Println("Private Location Container Stopped")

	imageproof := exec.Command("podman", "ps")
	output, _, err := runner(imageproof, "Cannot display podman ps")
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return output, http.StatusOK, nil
}

// NOTE: dead code (managing containers with systemd)
func containerNameList() []string {
	bufferSpace := bytes.NewBufferString("")
	var containerNames ContainerConfig
	err := json.Unmarshal([]byte(bufferSpace.String()), &containerNames)
	if err != nil {
		log.Println(err)
	}

	var nameList []string
	cnames := containerNames.Names
	for _, name := range cnames {
		nameList = append(nameList, name)
	}
	return nameList
}

func serviceActions(action string, name string) (int, any) {
	switch action {
	case "stop":
		serviceStop := exec.Command("systemctl", "stop", "container-"+name+".service")
		output, _, err := runner(serviceStop, fmt.Sprintf("Failed to stop private location %s", name))
		if err != nil {
			log.Println(err)
			return http.StatusInternalServerError, nil
		}
		log.Println(output)

		ImageRemover()

	case "pull":
		podmanPullImage()

	case "remove":
		ImageRemover()

	case "pp":
		pushPull()

	case "restart":
		serviceRestart := exec.Command("systemctl", "restart", "container-"+name+".service")
		output, _, err := runner(serviceRestart, fmt.Sprintf("Failed to restart private location %s", name))
		if err != nil {
			log.Println(err)
			return http.StatusInternalServerError, nil
		}
		return http.StatusOK, output
	}

	return http.StatusOK, nil
}
