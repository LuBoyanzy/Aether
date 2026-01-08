// docker_sdk_network.go 实现网络相关的 Docker SDK 操作。
// 包括网络列表、创建与删除。
package agent

import (
	"errors"
	"strings"
	"time"

	dockermodel "aether/internal/entities/docker"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

func (dm *dockerSDKManager) ListNetworks() ([]dockermodel.Network, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	list, err := dm.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	networks := make([]dockermodel.Network, 0, len(list))
	for _, item := range list {
		subnets, gateways := collectNetworkIPAM(item.IPAM.Config)
		networks = append(networks, dockermodel.Network{
			ID:         item.ID,
			Name:       item.Name,
			Driver:     item.Driver,
			Scope:      item.Scope,
			Internal:   item.Internal,
			Attachable: item.Attachable,
			Ingress:    item.Ingress,
			EnableIPv6: item.EnableIPv6,
			Labels:     item.Labels,
			Subnets:    subnets,
			Gateways:   gateways,
			Created:    formatDockerNetworkCreated(item.Created),
		})
	}
	return networks, nil
}

func formatDockerNetworkCreated(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func collectNetworkIPAM(configs []network.IPAMConfig) ([]string, []string) {
	subnets := make([]string, 0, len(configs))
	gateways := make([]string, 0, len(configs))
	subnetSet := make(map[string]struct{})
	gatewaySet := make(map[string]struct{})
	for _, config := range configs {
		if config.Subnet != "" {
			if _, exists := subnetSet[config.Subnet]; !exists {
				subnetSet[config.Subnet] = struct{}{}
				subnets = append(subnets, config.Subnet)
			}
		}
		if config.Gateway != "" {
			if _, exists := gatewaySet[config.Gateway]; !exists {
				gatewaySet[config.Gateway] = struct{}{}
				gateways = append(gateways, config.Gateway)
			}
		}
	}
	return subnets, gateways
}

func (dm *dockerSDKManager) CreateNetwork(req network.CreateOptions, name string) error {
	if err := dm.ensureAvailable(); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("network name is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	_, err := dm.client.NetworkCreate(ctx, name, req)
	return err
}

func (dm *dockerSDKManager) RemoveNetwork(networkID string) error {
	if err := dm.ensureAvailable(); err != nil {
		return err
	}
	if strings.TrimSpace(networkID) == "" {
		return errors.New("network id is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	return dm.client.NetworkRemove(ctx, networkID)
}

func (dm *dockerSDKManager) NetworkExists(name string) (bool, error) {
	if err := dm.ensureAvailable(); err != nil {
		return false, err
	}
	if strings.TrimSpace(name) == "" {
		return false, errors.New("network name is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	args := filters.NewArgs(filters.Arg("name", name))
	list, err := dm.client.NetworkList(ctx, network.ListOptions{Filters: args})
	if err != nil {
		return false, err
	}
	return len(list) > 0, nil
}
