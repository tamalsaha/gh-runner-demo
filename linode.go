package main

import (
	"context"
	"fmt"
	"github.com/linode/linodego"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"gomodules.xyz/pointer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	_ "k8s.io/klog/v2"
	"time"

	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	RetryInterval    = 50 * time.Millisecond
	RetryTimeout     = 2 * time.Second
	ReadinessTimeout = 10 * time.Minute
	GCTimeout        = 5 * time.Minute
)

var errLBNotFound = errors.New("loadbalancer not found")

type cloudConnector struct {
	client *linodego.Client
	region string
}

func NewLinodeClient() *linodego.Client {
	token := os.Getenv("LINODE_CLI_TOKEN")
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	c := linodego.NewClient(oauth2Client)
	return &c
}

func NewConnector() *cloudConnector {
	return &cloudConnector{
		client: NewLinodeClient(),
		region: "us-east",
	}
}

func (conn *cloudConnector) waitForStatus(id int, status linodego.InstanceStatus) error {
	attempt := 0
	klog.Info("waiting for instance status", "status", status)
	return wait.PollImmediate(RetryInterval, RetryTimeout, func() (bool, error) {
		attempt++

		instance, err := conn.client.GetInstance(context.Background(), id)
		if err != nil {
			return false, nil
		}
		if instance == nil {
			return false, nil
		}
		klog.Info("current instance state", "instance", instance.Label, "status", instance.Status, "attempt", attempt)
		if instance.Status == status {
			klog.Info("current instance status", "status", status)
			return true, nil
		}
		return false, nil
	})
}

func (conn *cloudConnector) getStartupScriptID() (int, error) {
	scriptName := "gh-runner-script"
	filter := fmt.Sprintf(`{"label" : "%v"}`, scriptName)
	listOpts := &linodego.ListOptions{PageOptions: nil, Filter: filter}

	scripts, err := conn.client.ListStackscripts(context.Background(), listOpts)
	if err != nil {
		return 0, err
	}

	if len(scripts) > 1 {
		return 0, errors.Errorf("multiple stackscript found with label %v", scriptName)
	} else if len(scripts) == 0 {
		return 0, errors.Errorf("no stackscript found with label %v", scriptName)
	}
	return scripts[0].ID, nil
}

//nolint:unparam
func (conn *cloudConnector) createOrUpdateStackScript(machine *clusterapi.Machine, script string) (int, error) {
	machineConfig, err := linode_config.MachineConfigFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return 0, err
	}
	scriptName := "gh-runner-script"
	filter := fmt.Sprintf(`{"label" : "%v"}`, scriptName)
	listOpts := &linodego.ListOptions{PageOptions: nil, Filter: filter}

	scripts, err := conn.client.ListStackscripts(context.Background(), listOpts)
	if err != nil {
		return 0, err
	}

	if len(scripts) > 1 {
		return 0, errors.Errorf("multiple stackscript found with label %v", scriptName)
	} else if len(scripts) == 0 {
		createOpts := linodego.StackscriptCreateOptions{
			Label:       scriptName,
			Description: fmt.Sprintf("Startup script for NodeGroup %s of Cluster %s", machine.Name, conn.Cluster.Name),
			Images:      []string{conn.Cluster.ClusterConfig().Cloud.InstanceImage},
			Script:      script,
		}
		stackScript, err := conn.client.CreateStackscript(context.Background(), createOpts)
		if err != nil {
			return 0, err
		}
		klog.Info("Stack script created", "role", string(machineConfig.Roles[0]))
		return stackScript.ID, nil
	}

	updateOpts := scripts[0].GetUpdateOptions()
	updateOpts.Script = script

	stackScript, err := conn.client.UpdateStackscript(context.Background(), scripts[0].ID, updateOpts)
	if err != nil {
		return 0, err
	}

	klog.Info("Stack script updated", "role", string(machineConfig.Roles[0]))
	return stackScript.ID, nil
}

func (conn *cloudConnector) DeleteStackScript() error {
	scriptName := "gh-runner-script"
	filter := fmt.Sprintf(`{"label" : "%v"}`, scriptName)
	listOpts := &linodego.ListOptions{PageOptions: nil, Filter: filter}

	scripts, err := conn.client.ListStackscripts(context.Background(), listOpts)
	if err != nil {
		return err
	}
	for _, script := range scripts {
		if err := conn.client.DeleteStackscript(context.Background(), script.ID); err != nil {
			return err
		}
	}
	return nil
}

func (conn *cloudConnector) CreateInstance(machine *clusterapi.Machine, script string) (*api.NodeInfo, error) {
	if _, err := conn.createOrUpdateStackScript(machine, script); err != nil {
		return nil, err
	}
	scriptID, err := conn.getStartupScriptID(machine)
	if err != nil {
		return nil, err
	}
	machineConfig, err := linode_config.MachineConfigFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}
	createOpts := linodego.InstanceCreateOptions{
		Label:    machine.Name,
		Region:   conn.region,
		Type:     machineConfig.Type,
		RootPass: conn.Cluster.ClusterConfig().Cloud.Linode.RootPassword,
		AuthorizedKeys: []string{
			string(conn.Certs.SSHKey.PublicKey),
		},
		StackScriptData: map[string]string{
			"GITHUB_TOKEN": machine.Name,
		},
		StackScriptID:  scriptID,
		Image:          conn.Cluster.ClusterConfig().Cloud.InstanceImage,
		BackupsEnabled: false,
		PrivateIP:      true,
		SwapSize:       pointer.IntP(0),
	}

	instance, err := conn.client.CreateInstance(context.Background(), createOpts)
	if err != nil {
		return nil, err
	}

	if err := conn.waitForStatus(instance.ID, linodego.InstanceRunning); err != nil {
		return nil, err
	}

	return &node, nil
}

func (conn *cloudConnector) DeleteInstanceByProviderID(providerID string) error {
	id, err := instanceIDFromProviderID(providerID)
	if err != nil {
		return err
	}

	if err := conn.client.DeleteInstance(context.Background(), id); err != nil {
		return err
	}

	klog.Info("Instance deleted", "instance-id", id)
	return nil
}

func instanceIDFromProviderID(providerID string) (int, error) {
	if providerID == "" {
		return 0, errors.New("providerID cannot be empty string")
	}

	split := strings.Split(providerID, "/")
	if len(split) != 3 {
		return 0, errors.Errorf("unexpected providerID format: %s, format should be: linode://12345", providerID)
	}

	// since split[0] is actually "linode:"
	if strings.TrimSuffix(split[0], ":") != UID {
		return 0, errors.Errorf("provider name from providerID should be linode: %s", providerID)
	}

	return strconv.Atoi(split[2])
}

func (conn *cloudConnector) instanceIfExists(machine *clusterapi.Machine) (*linodego.Instance, error) {
	lds, err := conn.client.ListInstances(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	for _, ld := range lds {
		if ld.Label == machine.Name {
			return &ld, nil
		}
	}

	return nil, fmt.Errorf("no vm found with %v name", machine.Name)
}
