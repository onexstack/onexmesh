// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package eureka

import (
	"fmt"
	"strings"
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func init() {
	registry.RegisterRegistrar("eureka", func(opts any) (registry.Registrar, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("eureka: options must be eureka.Options, got %T", opts)
		}
		return NewRegistrar(o)
	})
	registry.RegisterDiscovery("eureka", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("eureka: options must be eureka.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}

// eurekaPort models Jackson's PortWrapper serialization {"$": n, "@enabled": ...}.
type eurekaPort struct {
	Value   int    `json:"$"`
	Enabled string `json:"@enabled"`
}

// eurekaDataCenterInfo identifies the data center of the instance.
type eurekaDataCenterInfo struct {
	Class string `json:"@class"`
	Name  string `json:"name"`
}

// eurekaLeaseInfo sets the lease renewal/duration for the instance.
type eurekaLeaseInfo struct {
	RenewalIntervalInSecs int `json:"renewalIntervalInSecs"`
	DurationInSecs        int `json:"durationInSecs"`
}

// eurekaInstance is the JSON representation of an Eureka instance.
type eurekaInstance struct {
	HostName         string               `json:"hostName"`
	App              string               `json:"app"`
	IPAddr           string               `json:"ipAddr"`
	VIPAddress       string               `json:"vipAddress"`
	SecureVIPAddress string               `json:"secureVipAddress"`
	Status           string               `json:"status"`
	Port             eurekaPort           `json:"port"`
	SecurePort       eurekaPort           `json:"securePort"`
	HealthCheckURL   string               `json:"healthCheckUrl,omitempty"`
	StatusPageURL    string               `json:"statusPageUrl,omitempty"`
	HomePageURL      string               `json:"homePageUrl,omitempty"`
	DataCenterInfo   eurekaDataCenterInfo `json:"dataCenterInfo"`
	Metadata         map[string]string    `json:"metadata,omitempty"`
	InstanceID       string               `json:"instanceId,omitempty"`
	LeaseInfo        *eurekaLeaseInfo     `json:"leaseInfo,omitempty"`
}

// eurekaRegisterRequest wraps the instance for the registration endpoint.
type eurekaRegisterRequest struct {
	Instance eurekaInstance `json:"instance"`
}

// eurekaApplications is the top-level discovery response.
type eurekaApplications struct {
	Applications eurekaApplicationList `json:"applications"`
}

type eurekaApplicationList struct {
	Application []eurekaApplication `json:"application"`
}

type eurekaApplication struct {
	Name     string           `json:"name"`
	Instance []eurekaInstance `json:"instance"`
}

// ttl returns the effective lease renewal interval, defaulting to 30s.
func ttl(opts Options) time.Duration {
	if opts.TTL <= 0 {
		return 30 * time.Second
	}
	return opts.TTL
}

// serverURL returns the Eureka server base URL, defaulting to the common
// localhost endpoint.
func serverURL(opts Options) string {
	if opts.ServerURL == "" {
		return "http://127.0.0.1:8761/eureka"
	}
	return strings.TrimRight(opts.ServerURL, "/")
}
