// Copyright © 2018 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package builder

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/clearlinux/mixer-tools/helpers"
)

// GetHostAndUpstreamFormats retreives the formats for the host and the mix's
// upstream version. It attempts to determine the format for the host machine,
// and if successful, looks up the format for the desired upstream version.
func (b *Builder) GetHostAndUpstreamFormats(upstreamVer string) (string, string, error) {
	// Determine the host's format
	hostFormat, err := ioutil.ReadFile("/usr/share/defaults/swupd/format")
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	// Get the upstream version
	if upstreamVer == "" {
		if err := b.ReadVersions(); err != nil {
			return "", "", errors.Wrap(err, "Unable to determine upstream version")
		}
		upstreamVer = b.UpstreamVer
	} else if upstreamVer == "latest" {
		ver, err := b.getLatestUpstreamVersion()
		if err != nil {
			return "", "", err
		}
		upstreamVer = ver
	}

	upstreamFormat, _, _, err := b.getUpstreamFormatRange(upstreamVer)
	if err != nil {
		return "", "", err
	}
	
	return string(hostFormat), upstreamFormat, nil
}


const Dockerfile = `FROM scratch
ADD mixer.tar.xz /
CMD ["/bin/bash"]`

func (b *Builder) generateDockerBase(bundles bundleSet, ver string, baseDir string) error {
	dockerChroot, err := ioutil.TempDir(baseDir, "chroot-")
	if err != nil {
		errors.Errorf("Failed to generate temporary docker image chroot: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(dockerChroot)
	}()

	bundlePath := filepath.Join(dockerChroot, "usr/share/clear/bundles")
	os.MkdirAll(bundlePath, 0755)
	for name, _ := range bundles {
		// Touch the bundle file
		f, err := os.Create(filepath.Join(bundlePath, name))
		if err != nil {
			return errors.Wrapf(err, "Failed to touch bundle file for %q", name)
		}
		f.Close()

	}

	// Build the update url
	end, err := url.Parse("/update")
	if err != nil {
		return err
	}
	base, err := url.Parse(b.UpstreamURL)
	if err != nil {
		return err
	}
	upstateURL := base.ResolveReference(end).String()

	// "swupd install" the contents for the chroot
	fmt.Println("Generating docker image content")
	cmdString := fmt.Sprintf("swupd verify --install --path=%s -m %s -u %s -F staging -S %s", 
		dockerChroot, 
		ver,
		upstateURL,
		filepath.Join(dockerChroot, "swupd-state"),
		)
	cmd := strings.Split(cmdString, " ")
	err = helpers.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		return errors.Wrap(err, "Error creating docker image chroot")
	}

	// Tar up the dockerfile chroot
	fmt.Println("Compressing docker image chroot")
	cmdString = fmt.Sprintf("tar -C %s -cJf %s .", dockerChroot, filepath.Join(baseDir, "dockerbase.tar.xz"))
	cmd = strings.Split(cmdString, " ")
	err = helpers.RunCommand(cmd[0], cmd[1:]...)
	if err != nil {
		return errors.Wrap(err, "Error compressing docker image chroot")
	}

	return nil
}

func (b *Builder) fetchDockerBase(ver string, baseDir string) error {
	fmt.Println("got here 1.1")
	filename := filepath.Join(baseDir, "mixer.tar.xz")
	// Return if already exists
	if _, err := os.Stat(filename); err == nil {
		// TODO: Check if checksum/sig matches (once base images are signed)
		fmt.Println("File already exists; skipping download")
		return nil
	}
	fmt.Println("got here 1.2")
	
	// TODO: Remove this once mixer image is published with releases
	url := "https://clr-jenkins.ostc.intel.com/job/create-docker-mixer/ws/mixer.tar.xz"
	if err := helpers.Download(url, filename); err != nil {
		return errors.Wrap(err, "Failed to download internal temporary docker base")
	}
	return nil

	
	// Download the mixer base image from upstream
	upstreamFile := fmt.Sprintf("/releases/%s/clear/mixer.tar.xz", ver)
	if err := b.DownloadFileFromUpstream(upstreamFile, filename); err != nil {
		return errors.Wrapf(err, "Failed to download docker image base for ver %s", ver)
	}

	return nil
}

func createDockerfile(dir string) error {
	filename := filepath.Join(dir, "Dockerfile")

	// Return if already exists
	if _, err := os.Stat(filename); err == nil {
		return nil
	}

	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrap(err, "Failed to create Dockerfile")
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Write([]byte(Dockerfile))
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) buildDockerImage(format, ver string) error {
	// Make docker root dir
	wd, _ := os.Getwd()
	dockerRoot := filepath.Join(wd, fmt.Sprintf("docker/mixer-%s", format))
	if err := os.MkdirAll(dockerRoot, 0777); err != nil {
		errors.Wrapf(err, "Failed to generate docker work dir: %s", dockerRoot)
	}
	
	// TODO: Check if docker image already exists and return early

	fmt.Println("got here 1")

	// Fetch docker image base
	if err := b.fetchDockerBase(ver, dockerRoot); err != nil {
		return errors.Wrap(err, "Error fetching Docker image base")
	}

	fmt.Println("got here 2")
	// Generate Dockerfile
	if err := createDockerfile(dockerRoot); err != nil {
		return err
	}
	fmt.Println("got here 3")
	// Build Docker image
	cmd := []string{
		"docker",
		"build",
		"-t", fmt.Sprintf("mixer-tools/mixer:%v"),
		"--rm",
		"-f", filepath.Join(dockerRoot,"Dockerfile"),
	}
	if err := helpers.RunCommand(cmd[0],cmd[1:]...); err != nil {
		return errors.Wrapf(err, "Failed to build dockerfile with command %q", strings.Join(cmd, " "))
	}

	return nil
}

func (b *Builder) RunCommandInContainer(cmd string) error {
	format, first, _, err := b.getUpstreamFormatRange(b.UpstreamVer)
	if err != nil {
		return err
	}


	if err := b.buildDockerImage(format, fmt.Sprint(first)); err != nil {
		return err
	}

	// Run command
	fmt.Printf("Running command in container: %v", cmd)

	return nil
}


func (b *Builder) Docker(bundles []string, ver string) error {
	// Make bundleset for bundles
	set := make(bundleSet)
	for _, name := range bundles {
		bundle, err := b.getBundleFromName(name)
		if err != nil {
			return err
		}
		set[name] = bundle
	}
	fullSet, err := b.getFullBundleSet(set)
	if err != nil {
		return err
	}

	// Make tmpdir
	wd, _ := os.Getwd()
	dockerRoot, err := ioutil.TempDir(wd, "docker-")
	if err != nil {
		errors.Errorf("Failed to generate temporary docker dirt: %s", err)
	}

	err = b.generateDockerBase(fullSet, ver, dockerRoot)
	if err != nil {
		return err
	}

	err = createDockerfile(dockerRoot)
	if err != nil {
		return err
	}

	return nil
}
