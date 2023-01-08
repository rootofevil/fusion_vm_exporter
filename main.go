package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-vm/vmware"
	"github.com/go-vm/vmware/vmrun"
)

type FusionVM struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	UDID      string         `json:"serial"`
	Arch      string         `json:"architechture"`
	IPAddress string         `json:"ip"`
	Version   string         `json:"version"`
	Platform  string         `json:"platform"`
	Object    *vmware.Fusion `json:"-"`
}

func main() {
	var vmDir string
	// vmDir = "/Users/roe/Virtual Machines.localized"
	flag.StringVar(&vmDir, "p", "", "")
	flag.Parse()
	vms, err := Discover(vmDir)
	if err != nil {
		log.Fatal(err)
	}
	ser, err := json.MarshalIndent(vms, " ", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(ser))
}

func Discover(path string) ([]*FusionVM, error) {
	vmxList, err := listVMX(path)
	if err != nil {
		return nil, err
	}
	res := []*FusionVM{}
	for _, vmx := range vmxList {
		vm := initFusionVM(vmx, "user", "password")
		if err := vm.setSerial(); err != nil {
			log.Println(err)
			continue
		}
		err := vm.setProps()
		if err != nil {
			log.Println(err)
			continue
		}
		res = append(res, vm)
	}
	return res, nil
}

func initFusionVM(vmx string, user string, password string) *FusionVM {
	fusion := vmware.NewFusion(vmx, "user", "password")
	return &FusionVM{
		Object: fusion,
		Path:   vmx,
	}
}

func (vm *FusionVM) setProps() error {
	err := vm.setIpAddress()
	if err != nil {
		return err
	}
	err = vm.setName()
	if err != nil {
		return err
	}
	err = vm.setPropsFromDetailedData()
	if err != nil {
		return err
	}
	return nil
}

func (vm *FusionVM) setName() error {
	name, err := vm.getProperty("displayName")
	if err != nil {
		return err
	}
	vm.Name = name
	return nil
}

func (vm FusionVM) getProperty(name string) (string, error) {
	return vm.Object.ReadVariable(vmrun.RuntimeConfig, name)
}

func (vm FusionVM) getDetailedData() (map[string]string, error) {
	line, err := vm.getProperty("guestInfo.detailed.data")
	if err != nil {
		return nil, err
	}
	propsMap := make(map[string]string)
	for _, p := range strings.Split(line, "' ") {
		data := strings.Split(p, "='")
		propsMap[data[0]] = data[1]
	}
	return propsMap, nil
}

func (vm *FusionVM) setSerial() error {
	copyto := filepath.Join(filepath.Dir(vm.Path), "serial.txt")
	if _, err := os.Stat(copyto); err != nil {
		vm.getSerial(copyto)
	}

	f, err := ioutil.ReadFile(copyto)
	if err != nil {
		return err
	}
	vm.UDID = strings.Trim(string(f), "\n")
	return nil
}

func (vm FusionVM) getSerial(copyto string) error {
	filename := "/tmp/serial"
	// "/usr/sbin/ioreg -d2 -c IOPlatformExpertDevice | /usr/bin/awk -F\" '/IOPlatformSerialNumber/{print $(NF-1)}' > %s"
	cmd := fmt.Sprintf("/usr/sbin/ioreg -d2 -c IOPlatformExpertDevice | /usr/bin/awk -F\\\" '/IOPlatformSerialNumber/{print $(NF-1)}' > %s", filename)
	err := vm.Object.RunScriptInGuest(0, "/bin/bash", cmd)
	if err != nil {
		return err
	}
	err = vm.Object.CopyFileFromGuestToHost(filename, copyto)
	if err != nil {
		return err
	}
	return nil
}

func (vm *FusionVM) setPropsFromDetailedData() error {
	data, err := vm.getDetailedData()
	if err != nil {
		return err
	}
	// for n, v := range data {
	// 	fmt.Printf("%s: %s\n", n, v)
	// }
	if arch, ok := data["architecture"]; ok {
		if bit, ok := data["bitness"]; ok {
			arch = arch + "_" + bit
		}
		vm.Arch = arch
	}
	if ver, ok := data["distroVersion"]; ok {
		vm.Version = ver
	}
	if platform, ok := data["distroName"]; ok {
		vm.Platform = platform
	}
	return nil
}

func (vm *FusionVM) GetIpAddress() string {
	return vm.IPAddress
}

func (vm *FusionVM) setIpAddress() error {
	ip, err := vm.Object.GetGuestIPAddress(true)
	if err != nil {
		return err
	}
	vm.IPAddress = strings.Trim(ip, "\n")
	return nil
}

func listVMX(path string) (listVMs []string, err error) {
	info, _ := os.Stat(path)
	if info.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if filepath.Ext(f.Name()) == ".vmwarevm" && f.IsDir() {
				basename := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
				vmx := filepath.Join(path, f.Name(), basename+".vmx")
				listVMs = append(listVMs, vmx)
			}
		}
	}
	return
}
