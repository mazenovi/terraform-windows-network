package main

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"

	"github.com/masterzen/winrm"
)

type WinrmError struct {
	codeReturn int
	message    string
	stderr     string
}

func (e *WinrmError) Error() string {
	return fmt.Sprintf("Winrm (%d) - %s", e.codeReturn, e.message)
}

type Communicator struct {
	username string
	password string
	client   *winrm.Client
	endpoint *winrm.Endpoint
}

func (c *Communicator) Connect() error {
	params := winrm.DefaultParameters
	client, err := winrm.NewClientWithParameters(c.endpoint, c.username, c.password, params)
	if err != nil {
		return err
	}
	shell, err := client.CreateShell()
	if err != nil {
		// error here if cannot connect
		return err
	}
	shell.Close()
	c.client = client
	return nil
}

func (c *Communicator) AddFilterAllowAddress(mac string, description string) error {
	command := fmt.Sprintf(
		"Add-DhcpServerv4Filter -List Allow -macAddress \"%s\" -Description \"%s\"",
		mac,
		description,
	)

	_, stderr, returnCode := c.Execute(command)

	if returnCode != 0 {
		return &WinrmError{returnCode, "Cannot allow mac address in dhcp, maybe already allowed.", stderr}
	}

	return nil
}

func (c *Communicator) RemoveFilterAllowAddress(mac string) error {
	command := fmt.Sprintf(
		"Remove-DhcpServerv4Filter \"%s\"", mac,
	)

	c.Execute(command)

	return nil
}

func (c *Communicator) GetAllAllowedMacAddress() []string {
	stdout, _, _ := c.Execute("Get-DhcpServerv4Filter -List Allow")
	lines := strings.Split(stdout, "\n")

	var macs []string

	re := regexp.MustCompile(`(([0-9ABCDEF]{2})-?){6,8}`)

	for _, element := range lines {
		matched, _ := regexp.MatchString(`^(([0-9ABCDEF]{2})-?){6,8}`, element)
		if matched {
			mac := string(re.Find([]byte(element)))
			macs = append(macs, mac)
		}
	}
	return macs
}

func (c *Communicator) AddBail(mac string, ip net.IP, scopeId string, description string, name string) error {

	command := fmt.Sprintf(
		"Add-DhcpServerv4Reservation -ScopeId %s -Description \"%s\" -IPAddress %s -Name %s -ClientId %s -Type Dhcp",
		scopeId, description, ip.String(), name, mac,
	)

	_, stderr, returnCode := c.Execute(command)

	if returnCode != 0 {
		return &WinrmError{returnCode, "Cannot add bail in dhcp server.", stderr}
	}

	return nil
}

func (c *Communicator) RemoveBail(mac string, scopeId string) error {

	command := fmt.Sprintf(
		"Remove-DhcpServerv4Reservation -ScopeId %s -ClientId \"%s\"",
		scopeId, mac,
	)

	c.Execute(command)

	return nil
}

func (c *Communicator) getFreeIp(scopeId string) (net.IP, error) {
	command := fmt.Sprintf(
		"Get-DhcpServerv4FreeIPAddress -ScopeId %s",
		scopeId,
	)

	ip, stderr, exitCode := c.Execute(command)
	if exitCode != 0 {
		return nil, &WinrmError{exitCode, "Cannot get a free ip.", stderr}
	}
	ipv4 := net.ParseIP(strings.TrimSpace(ip))
	log.Printf("[DEBUG] free ip for scope " + scopeId + " is " + ipv4.String())
	return ipv4, nil
}

func (c *Communicator) AddDNSRecordA(zone string, ip net.IP, name string) error {
	command := fmt.Sprintf(
		"Add-dnsserverresourcerecordA -name \"%s\" -zonename \"%s\" -allowupdateany -ipv4address \"%s\"",
		name, zone, ip.String(),
	)

	_, stderr, exitCode := c.Execute(command)

	if exitCode != 0 {
		return &WinrmError{exitCode, "Cannot add record A.", stderr}
	}

	return nil
}

func (c *Communicator) RemoveDNSRecordA(zone string, ip net.IP, name string) error {
	command := fmt.Sprintf(
		"Remove-DnsServerResourceRecord -zonename \"%s\" -RRType A -Name \"%s\" -RecordData \"%s\" -Force",
		zone, name, ip.String(),
	)

	c.Execute(command)

	return nil
}

func (c *Communicator) Execute(command string) (string, string, int) {
	stdout, stderr, returnCode, _ := c.client.RunWithString(winrm.Powershell(command), "")
	log.Printf(stdout)
	log.Printf(stderr)
	return stdout, stderr, returnCode
}
