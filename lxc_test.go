// Copyright © 2013, 2014, The Go-LXC Authors. All rights reserved.
// Use of this source code is governed by a LGPLv2.1
// license that can be found in the LICENSE file.

// +build linux,cgo

package lxc

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	DefaultContainerName             = "lorem"
	DefaultSnapshotName              = "snap0"
	DefaultContainerRestoreName      = "ipsum"
	DefaultContainerCloneName        = "consectetur"
	DefaultContainerCloneOverlayName = "adipiscing"
)

func exists(name string) bool {
	_, err := os.Lstat(name)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func unprivileged() bool {
	return os.Geteuid() != 0
}

func travis() bool {
	// https://docs.travis-ci.com/user/environment-variables/#default-environment-variables
	return os.Getenv("TRAVIS") == "true"
}

func supported(moduleName string) bool {
	if _, err := os.Stat("/sys/module/" + moduleName); err != nil {
		return false
	}
	return true
}

func ipv6() bool {
	lxcbr0, err := net.InterfaceByName("lxcbr0")
	if err != nil {
		return false
	}

	addresses, err := lxcbr0.Addrs()
	if err != nil {
		return false
	}

	// https://github.com/asaskevich/govalidator/blob/master/validator.go#L621
	for _, v := range addresses {
		if ipnet, ok := v.(*net.IPNet); ok && strings.Count(v.String(), ":") >= 2 && !ipnet.IP.IsLinkLocalUnicast() {
			return true
		}
	}

	return false
}

func template() TemplateOptions {
	return TemplateOptions{
		Template: "download",
		Distro:   "alpine",
		Release:  "edge",
		Arch:     "amd64",
	}
}

func ContainerName() string {
	if unprivileged() {
		return fmt.Sprintf("%s-unprivileged", DefaultContainerName)
	}
	return DefaultContainerName
}

func ContainerRestoreName() string {
	if unprivileged() {
		return fmt.Sprintf("%s-unprivileged", DefaultContainerRestoreName)
	}
	return DefaultContainerRestoreName
}

func ContainerCloneName() string {
	if unprivileged() {
		return fmt.Sprintf("%s-unprivileged", DefaultContainerCloneName)
	}
	return DefaultContainerCloneName
}

func ContainerCloneOverlayName() string {
	if unprivileged() {
		return fmt.Sprintf("%s-unprivileged", DefaultContainerCloneOverlayName)
	}
	return DefaultContainerCloneOverlayName
}

func TestVersion(t *testing.T) {
	t.Logf("LXC version: %s", Version())
}

func TestDefaultConfigPath(t *testing.T) {
	if DefaultConfigPath() == "" {
		t.Errorf("DefaultConfigPath failed...")
	}
}

func TestSetConfigPath(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	currentPath := c.ConfigPath()
	if err := c.SetConfigPath("/tmp"); err != nil {
		t.Errorf(err.Error())
	}
	newPath := c.ConfigPath()

	if currentPath == newPath {
		t.Errorf("SetConfigPath failed...")
	}
}

func TestAcquire(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	Acquire(c)
	Release(c)
}

func TestConcurrentDefined_Negative(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	for i := 0; i <= 100; i++ {
		wg.Add(1)
		go func() {
			c, err := NewContainer(strconv.Itoa(rand.Intn(10)))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			// sleep for a while to simulate some work
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(250)))

			if c.Defined() {
				t.Errorf("Defined_Negative failed...")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestDefined_Negative(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.Defined() {
		t.Errorf("Defined_Negative failed...")
	}
}

func TestExecute(t *testing.T) {
	if unprivileged() {
		t.Skip("skipping test in unprivileged mode.")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	c.SetConfigItem("lxc.apparmor.profile", "unconfined")
	if _, err := c.Execute("/bin/true"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSetVerbosity(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	c.SetVerbosity(Quiet)
}

func TestCreate(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	c.SetVerbosity(Verbose)

	if err := c.Create(template()); err != nil {
		t.Errorf(err.Error())
	}
}

func TestClone(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err = c.Clone(ContainerCloneName(), DefaultCloneOptions); err != nil {
		t.Errorf(err.Error())
	}
}

func TestCloneUsingOverlayfs(t *testing.T) {
	if !(supported("overlayfs") || supported("overlay")) {
		t.Skip("skipping test as overlayfs support is missing.")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	err = c.Clone(ContainerCloneOverlayName(), CloneOptions{
		Backend:  Overlayfs,
		KeepName: true,
		KeepMAC:  true,
		Snapshot: true,
	})
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestCreateSnapshot(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.CreateSnapshot(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestCreateSnapshots(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	for i := 0; i < 3; i++ {
		if _, err := c.CreateSnapshot(); err != nil {
			t.Errorf(err.Error())
		}
	}
}

func TestRestoreSnapshot(t *testing.T) {
	if os.Getenv("GITHUB_ACTION") != "" {
		t.Skip("Test broken on Github")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	snapshot := Snapshot{Name: DefaultSnapshotName}
	if err := c.RestoreSnapshot(snapshot, ContainerRestoreName()); err != nil {
		t.Errorf(err.Error())
	}
}

func TestConcurrentCreate(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	options := template()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			c, err := NewContainer(strconv.Itoa(i))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			// sleep for a while to simulate some work
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(250)))

			if err := c.Create(options); err != nil {
				t.Errorf(err.Error())
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestSnapshots(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.Snapshots(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestConcurrentStart(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			c, err := NewContainer(strconv.Itoa(i))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			if err := c.Start(); err != nil {
				t.Errorf(err.Error())
			}

			c.Wait(RUNNING, 30*time.Second)
			if !c.Running() {
				t.Errorf("Starting the container failed...")
			}

			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestConfigFileName(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.ConfigFileName() == "" {
		t.Errorf("ConfigFileName failed...")
	}
}

func TestDefined_Positive(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if !c.Defined() {
		t.Errorf("Defined_Positive failed...")
	}
}

func TestConcurrentDefined_Positive(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	for i := 0; i <= 100; i++ {
		wg.Add(1)
		go func() {
			c, err := NewContainer(strconv.Itoa(rand.Intn(10)))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			// sleep for a while to simulate some work
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(250)))

			if !c.Defined() {
				t.Errorf("Defined_Positive failed...")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestInitPid_Negative(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.InitPid() != -1 {
		t.Errorf("InitPid failed...")
	}
}

func TestStart(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	log := fmt.Sprintf("/tmp/%s", ContainerName())
	if err := c.SetLogFile(log); err != nil {
		t.Errorf("SetLogFile failed...")
	}

	if err := c.Start(); err != nil {
		t.Errorf(err.Error())
	}

	c.Wait(RUNNING, 30*time.Second)
	if !c.Running() {
		t.Errorf("Starting the container failed...")

		b, err := ioutil.ReadFile(log)
		if err != nil {
			t.Errorf("Reading %s file failed...", log)
		}
		t.Logf("%s\n", b)
	}
}

func TestWaitIPAddresses(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.WaitIPAddresses(30 * time.Second); err != nil {
		t.Errorf(err.Error())
	}
}

func TestControllable(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if !c.Controllable() {
		t.Errorf("Controlling the container failed...")
	}
}

func TestContainerNames(t *testing.T) {
	if ContainerNames() == nil {
		t.Errorf("ContainerNames failed...")
	}
}

func TestDefinedContainerNames(t *testing.T) {
	if DefinedContainerNames() == nil {
		t.Errorf("DefinedContainerNames failed...")
	}
}

func TestActiveContainerNames(t *testing.T) {
	if ActiveContainerNames() == nil {
		t.Errorf("ActiveContainerNames failed...")
	}
}

func TestContainers(t *testing.T) {
	if Containers() == nil {
		t.Errorf("Containers failed...")
	}
}

func TestDefinedContainers(t *testing.T) {
	if DefinedContainers() == nil {
		t.Errorf("DefinedContainers failed...")
	}
}

func TestActiveContainers(t *testing.T) {
	if ActiveContainers() == nil {
		t.Errorf("ActiveContainers failed...")
	}
}

func TestRunning(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if !c.Running() {
		t.Errorf("Checking the container failed...")
	}
}

func TestWantDaemonize(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.WantDaemonize(false); err != nil || c.Daemonize() {
		t.Errorf("WantDaemonize failed...")
	}
}

func TestWantCloseAllFds(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.WantCloseAllFds(true); err != nil {
		t.Errorf("WantCloseAllFds failed...")
	}
}

func TestSetLogLevel(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.SetLogLevel(WARN); err != nil || c.LogLevel() != WARN {
		t.Errorf("SetLogLevel( failed...")
	}
}

func TestSetLogFile(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.SetLogFile("/tmp/" + ContainerName()); err != nil || c.LogFile() != "/tmp/"+ContainerName() {
		t.Errorf("SetLogFile failed...")
	}
}

func TestInitPid_Positive(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.InitPid() == -1 {
		t.Errorf("InitPid failed...")
	}
}

func TestName(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.Name() != ContainerName() {
		t.Errorf("Name failed...")
	}
}

func TestFreeze(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Freeze(); err != nil {
		t.Errorf(err.Error())
	}

	c.Wait(FROZEN, 30*time.Second)
	if c.State() != FROZEN {
		t.Errorf("Freezing the container failed...")
	}
}

func TestUnfreeze(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Unfreeze(); err != nil {
		t.Errorf(err.Error())
	}

	c.Wait(RUNNING, 30*time.Second)
	if !c.Running() {
		t.Errorf("Unfreezing the container failed...")
	}
}

func TestLoadConfigFile(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.LoadConfigFile(c.ConfigFileName()); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSaveConfigFile(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.SaveConfigFile(c.ConfigFileName()); err != nil {
		t.Errorf(err.Error())
	}
}

func TestConfigItem(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.ConfigItem("lxc.uts.name")[0] != ContainerName() {
		t.Errorf("ConfigItem failed...")
	}
}

func TestSetConfigItem(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.SetConfigItem("lxc.uts.name", ContainerName()); err != nil {
		t.Errorf(err.Error())
	}

	if c.ConfigItem("lxc.uts.name")[0] != ContainerName() {
		t.Errorf("ConfigItem failed...")
	}
}

func TestRunningConfigItem(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.RunningConfigItem("lxc.network.0.type") == nil {
		t.Errorf("RunningConfigItem failed...")
	}
}

func TestSetCgroupItem(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	maxMem := c.CgroupItem("memory.max_usage_in_bytes")[0]
	currentMem := c.CgroupItem("memory.limit_in_bytes")[0]
	if err := c.SetCgroupItem("memory.limit_in_bytes", maxMem); err != nil {
		t.Errorf(err.Error())
	}
	newMem := c.CgroupItem("memory.limit_in_bytes")[0]

	if newMem == currentMem {
		t.Errorf("SetCgroupItem failed...")
	}
}

func TestClearConfigItem(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.ClearConfigItem("lxc.cap.drop"); err != nil {
		t.Errorf(err.Error())
	}
	if c.ConfigItem("lxc.cap.drop")[0] != "" {
		t.Errorf("ClearConfigItem failed...")
	}
}

func TestConfigKeys(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	keys := ""
	if VersionAtLeast(2, 1, 0) {
		keys = strings.Join(c.ConfigKeys("lxc.net.0"), " ")
	} else {
		keys = strings.Join(c.ConfigKeys("lxc.network.0"), " ")
	}

	if !strings.Contains(keys, "mtu") {
		t.Errorf("Keys failed...")
	}
}

func TestInterfaces(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.Interfaces(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestInterfaceStats(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.InterfaceStats(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestMemoryUsage(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.MemoryUsage(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestKernelMemoryUsage(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.KernelMemoryUsage(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestMemorySwapUsage(t *testing.T) {
	if !exists("/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes") {
		t.Skip("skipping the test as it requires memory.memsw.limit_in_bytes to be set")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.MemorySwapUsage(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestBlkioUsage(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.BlkioUsage(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestMemoryLimit(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.MemoryLimit(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSoftMemoryLimit(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.SoftMemoryLimit(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestKernelMemoryLimit(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.KernelMemoryLimit(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestMemorySwapLimit(t *testing.T) {
	if !exists("/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes") {
		t.Skip("skipping the test as it requires memory.memsw.limit_in_bytes to be set")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.MemorySwapLimit(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSetMemoryLimit(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	oldMemLimit, err := c.MemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	if err := c.SetMemoryLimit(oldMemLimit * 4); err != nil {
		t.Errorf(err.Error())
	}

	newMemLimit, err := c.MemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	if newMemLimit != oldMemLimit*4 {
		t.Errorf("SetMemoryLimit failed")
	}
}

func TestSetSoftMemoryLimit(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	oldMemLimit, err := c.MemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	if err := c.SetSoftMemoryLimit(oldMemLimit * 4); err != nil {
		t.Errorf(err.Error())
	}

	newMemLimit, err := c.SoftMemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	if newMemLimit != oldMemLimit*4 {
		t.Errorf("SetSoftMemoryLimit failed")
	}
}

func TestSetKernelMemoryLimit(t *testing.T) {
	t.Skip("skipping the test as it requires memory.kmem.limit_in_bytes to be set")

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	oldMemLimit, err := c.KernelMemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	if err := c.SetKernelMemoryLimit(oldMemLimit * 4); err != nil {
		t.Errorf(err.Error())
	}

	newMemLimit, err := c.KernelMemoryLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	// Floats aren't exactly exact, check that we did get something smaller
	if newMemLimit < oldMemLimit*3 {
		t.Errorf("SetKernelMemoryLimit failed")
	}
}

func TestSetMemorySwapLimit(t *testing.T) {
	if !exists("/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes") {
		t.Skip("skipping the test as it requires memory.memsw.limit_in_bytes to be set")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	oldMemorySwapLimit, err := c.MemorySwapLimit()
	if err != nil {
		t.Errorf(err.Error())
	}
	if err := c.SetMemorySwapLimit(oldMemorySwapLimit / 4); err != nil {
		t.Errorf(err.Error())
	}

	newMemorySwapLimit, err := c.MemorySwapLimit()
	if err != nil {
		t.Errorf(err.Error())
	}

	// Floats aren't exactly exact, check that we did get something smaller
	if newMemorySwapLimit > oldMemorySwapLimit/3 {
		t.Errorf("SetSwapLimit failed")
	}
}

func TestCPUTime(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.CPUTime(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestCPUTimePerCPU(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.CPUTimePerCPU(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestCPUStats(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.CPUStats(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestRunCommandNoWait(t *testing.T) {
	c, err := NewContainer("TestRunCommandNoWait")
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}
	defer c.Release()

	if err := c.Create(template()); err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}
	defer c.Destroy()

	err = c.Start()
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}
	defer c.Stop()

	argsThree := []string{"/bin/sh", "-c", "exit 0"}
	pid, err := c.RunCommandNoWait(argsThree, DefaultAttachOptions)
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	procState, err := proc.Wait()
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}
	if !procState.Success() {
		t.Errorf("Expected success")
		t.FailNow()
	}

	argsThree = []string{"/bin/sh", "-c", "exit 1"}
	pid, err = c.RunCommandNoWait(argsThree, DefaultAttachOptions)
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	proc, err = os.FindProcess(pid)
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	procState, err = proc.Wait()
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	if procState.Success() {
		t.Errorf("Expected failure")
		t.FailNow()
	}
}

func TestRunCommand(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	argsThree := []string{"/bin/sh", "-c", "exit 0"}
	ok, err := c.RunCommand(argsThree, DefaultAttachOptions)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}

	argsThree = []string{"/bin/sh", "-c", "exit 1"}
	ok, err = c.RunCommand(argsThree, DefaultAttachOptions)
	if err != nil {
		t.Errorf(err.Error())
	}
	if ok {
		t.Errorf("Expected failure")
	}
}

func TestCommandWithEnv(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	options := DefaultAttachOptions
	options.Env = []string{"FOO=BAR"}
	options.ClearEnv = true

	args := []string{"/bin/sh", "-c", "test $FOO = 'BAR'"}
	ok, err := c.RunCommand(args, options)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}
}

func TestCommandWithEnvToKeep(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	options := DefaultAttachOptions
	options.ClearEnv = true
	options.EnvToKeep = []string{"USER"}

	args := []string{"/bin/sh", "-c", fmt.Sprintf("test $USER = '%s'", os.Getenv("USER"))}
	ok, err := c.RunCommand(args, DefaultAttachOptions)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}
}

func TestCommandWithCwd(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	options := DefaultAttachOptions
	options.Cwd = "/tmp"

	args := []string{"/bin/sh", "-c", "test `pwd` = /tmp"}
	ok, err := c.RunCommand(args, options)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}
}

func TestCommandWithUIDGID(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	options := DefaultAttachOptions
	options.UID = 1000
	options.GID = 1000

	args := []string{"/bin/sh", "-c", "test `id -u` = 1000 && test `id -g` = 1000"}
	ok, err := c.RunCommand(args, options)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}
}

func TestCommandWithArch(t *testing.T) {
	uname := syscall.Utsname{}
	if err := syscall.Uname(&uname); err != nil {
		t.Errorf(err.Error())
	}

	arch := ""
	for _, c := range uname.Machine {
		if c == 0 {
			break
		}
		arch += string(byte(c))
	}

	if arch != "x86_64" && arch != "i686" {
		t.Skip("skipping architecture test, not on x86")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	options := DefaultAttachOptions
	options.Arch = X86

	args := []string{"/bin/sh", "-c", "test `uname -m` = i686"}
	ok, err := c.RunCommand(args, options)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !ok {
		t.Errorf("Expected success")
	}
}

func TestConsoleFd(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.ConsoleFd(0); err != nil {
		t.Errorf(err.Error())
	}
}

func TestIPAddress(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.IPAddress("lo"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestIPv4Address(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.IPv4Address("lo"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestIPv46ddress(t *testing.T) {
	if !ipv6() {
		t.Skip("skipping test since lxc bridge does not have ipv6 address")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.IPv6Address("lo"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestAddDeviceNode(t *testing.T) {
	if unprivileged() {
		t.Skip("skipping test in unprivileged mode.")
	}

	if !exists("/dev/network_latency") {
		t.Skip("skipping the test as it requires/dev/network_latency")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.AddDeviceNode("/dev/network_latency"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestRemoveDeviceNode(t *testing.T) {
	if unprivileged() {
		t.Skip("skipping test in unprivileged mode.")
	}

	if !exists("/dev/network_latency") {
		t.Skip("skipping the test as it requires/dev/network_latency")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.RemoveDeviceNode("/dev/network_latency"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestIPv4Addresses(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.IPv4Addresses(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestIPv6Addresses(t *testing.T) {
	if !ipv6() {
		t.Skip("skipping test since lxc bridge does not have ipv6 address")
	}

	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if _, err := c.IPv6Addresses(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestReboot(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Reboot(); err != nil {
		t.Errorf("Rebooting the container failed...")
	}
	c.Wait(RUNNING, 30*time.Second)
}

func TestConcurrentShutdown(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			c, err := NewContainer(strconv.Itoa(i))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			if err := c.Shutdown(30 * time.Second); err != nil {
				t.Errorf(err.Error())
			}

			c.Wait(STOPPED, 30*time.Second)
			if c.Running() {
				t.Errorf("Shutting down the container failed...")
			}

			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestShutdown(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Shutdown(30 * time.Second); err != nil {
		t.Errorf(err.Error())
	}

	c.Wait(STOPPED, 30*time.Second)
	if c.Running() {
		t.Errorf("Shutting down the container failed...")
	}
}

func TestStop(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Start(); err != nil {
		t.Errorf(err.Error())
	}

	if err := c.Stop(); err != nil {
		t.Errorf(err.Error())
	}

	c.Wait(STOPPED, 30*time.Second)
	if c.Running() {
		t.Errorf("Stopping the container failed...")
	}
}

func TestDestroySnapshot(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	snapshot := Snapshot{Name: DefaultSnapshotName}
	if err := c.DestroySnapshot(snapshot); err != nil {
		t.Errorf(err.Error())
	}
}

func TestDestroyAllSnapshots(t *testing.T) {
	c, err := NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.DestroyAllSnapshots(); err != nil {
		if err == ErrNotSupported {
			t.Skip("skipping due to lxc version.")
		}
		t.Errorf(err.Error())
	}
}

func TestDestroy(t *testing.T) {
	if supported("overlayfs") || supported("overlay") {
		c, err := NewContainer(ContainerCloneOverlayName())
		if err != nil {
			t.Errorf(err.Error())
		}
		defer c.Release()

		if err := c.Destroy(); err != nil {
			t.Errorf(err.Error())
		}
	}

	c, err := NewContainer(ContainerCloneName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Destroy(); err != nil {
		t.Errorf(err.Error())
	}

	c, err = NewContainer(ContainerRestoreName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if c.Defined() {
		if err := c.Destroy(); err != nil {
			t.Errorf(err.Error())
		}
	}

	c, err = NewContainer(ContainerName())
	if err != nil {
		t.Errorf(err.Error())
	}
	defer c.Release()

	if err := c.Destroy(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestConcurrentDestroy(t *testing.T) {
	t.Skip("Skipping concurrent tests for now")

	defer runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			c, err := NewContainer(strconv.Itoa(i))
			if err != nil {
				t.Errorf(err.Error())
			}
			defer c.Release()

			// sleep for a while to simulate some work
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(250)))

			if err := c.Destroy(); err != nil {
				t.Errorf(err.Error())
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestBackendStore(t *testing.T) {
	var X struct {
		store BackendStore
	}

	if X.store.String() != "" {
		t.Error("zero value of BackendStore should be invalid")
	}
}

func TestState(t *testing.T) {
	var X struct {
		state State
	}

	if X.state.String() != "" {
		t.Error("zero value of State should be invalid")
	}
}

func TestSupportedConfigItems(t *testing.T) {
	if VersionAtLeast(2, 1, 0) {
		if !IsSupportedConfigItem("lxc.arch") {
			t.Errorf("IsSupportedConfigItem failed to detect \"lxc.arch\" as supported config item...")
		}

		if IsSupportedConfigItem("lxc.nonsense") {
			t.Errorf("IsSupportedConfigItem failed to detect \"lxc.nonsense\" as unsupported config item...")
		}
	}
}

func TestRuntimeLiblxcVersionAtLeast(t *testing.T) {
	type args struct {
		version string
		major   int
		minor   int
		micro   int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Check 5.0.0 is at least 2.1.0 returns true",
			args: args{
				version: "5.0.0",
				major:   2,
				minor:   1,
				micro:   0,
			},
			want: true,
		},
		{
			name: "Check 5.0.0-devel is at least 2.1.0",
			args: args{
				version: "5.0.0-devel",
				major:   2,
				minor:   1,
				micro:   0,
			},
			want: true,
		},
		{
			name: "Check 5.0.0~git2209-g5a7b9ce67-0ubuntu1 is at least 2.1.0",
			args: args{
				version: "5.0.0~git2209-g5a7b9ce67-0ubuntu1",
				major:   2,
				minor:   1,
				micro:   0,
			},
			want: true,
		},
		{
			name: "Check 1.0.0 is not at least 2.1.0 returns true",
			args: args{
				version: "1.0.0",
				major:   2,
				minor:   1,
				micro:   0,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuntimeLiblxcVersionAtLeast(tt.args.version, tt.args.major, tt.args.minor, tt.args.micro); got != tt.want {
				t.Errorf("RuntimeLiblxcVersionAtLeast() = %v, want %v", got, tt.want)
			}
		})
	}
}
