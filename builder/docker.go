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

	"github.com/clearlinux/mixer-tools/helpers"
	"github.com/pkg/errors"
)

const Dockerfile = `FROM scratch
ADD dockerbase.tar.xz /
CMD ["/bin/bash"]`


func (b *Builder)generateDockerBase(bundles bundleSet, ver string, baseDir string) error {
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

	//Build the update url
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

func createDockerfile(dir string) error {
	f, err := os.Create(filepath.Join(dir, "Dockerfile"))
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
