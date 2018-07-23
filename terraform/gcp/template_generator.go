package gcp

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type templates struct {
	vars         string
	jumpbox      string
	boshDirector string
	cfLB         string
	cfDNS        string
	concourseLB  string
}

type TemplateGenerator struct{}

func NewTemplateGenerator() TemplateGenerator {
	return TemplateGenerator{}
}

func (t TemplateGenerator) Generate(state storage.State) string {
	tmpls := readTemplates()

	template := strings.Join([]string{tmpls.vars, tmpls.boshDirector, tmpls.jumpbox}, "\n")

	switch state.LB.Type {
	case "concourse":
		template = strings.Join([]string{template, tmpls.concourseLB}, "\n")
	case "cf":
		instanceGroups := t.GenerateInstanceGroups(state.GCP.Zones)
		backendService := t.GenerateBackendService(state.GCP.Zones)

		template = strings.Join([]string{template, tmpls.cfLB, instanceGroups, backendService}, "\n")

		if state.LB.Domain != "" {
			template = strings.Join([]string{template, tmpls.cfDNS}, "\n")
		}
	}

	cidrs := t.GenerateSubnets(state.GCP.Zones)
	if len(cidrs) > 0 {
		template = strings.Join([]string{template, cidrs}, "\n")
	}

	return template
}

func (t TemplateGenerator) GenerateBackendService(zoneList []string) string {
	backendBase := `resource "google_compute_backend_service" "router-lb-backend-service" {
  name        = "${var.env_id}-router-lb"
  port_name   = "https"
  protocol    = "HTTPS"
  timeout_sec = 900
  enable_cdn  = false
%s
  health_checks = ["${google_compute_health_check.cf-public-health-check.self_link}"]
}
`
	var backends string
	for i := 0; i < len(zoneList); i++ {
		backends = fmt.Sprintf(`%s
  backend {
    group = "${google_compute_instance_group.router-lb-%d.self_link}"
  }
`, backends, i)
	}

	return fmt.Sprintf(backendBase, backends)
}

func (t TemplateGenerator) GenerateInstanceGroups(zoneList []string) string {
	var groups []string
	for i, zone := range zoneList {
		groups = append(groups, fmt.Sprintf(`resource "google_compute_instance_group" "router-lb-%[1]d" {
  name        = "${var.env_id}-router-lb-%[1]d-%[2]s"
  description = "terraform generated instance group that is multi-zone for https loadbalancing"
  zone        = "%[2]s"

  named_port {
    name = "https"
    port = "443"
  }
}
`, i, zone))
	}

	return strings.Join(groups, "\n")
}

func (t TemplateGenerator) GenerateSubnets(zoneList []string) string {
	tmpl := `resource "google_compute_subnetwork" "bbl-subnet-%[1]d" {
  name          = "${var.env_id}-subnet-%[1]d"
  ip_cidr_range = "${cidrsubnet(var.subnet_cidr, 8, %[2]d)}"
  network       = "${google_compute_network.bbl-network.self_link}"
}

output "subnetwork_%[1]d" {
  value = "${google_compute_subnetwork.bbl-subnet-%[1]d.name}"
}

output "subnet_cidr_%[1]d" {
  value = "${google_compute_subnetwork.bbl-subnet-%[1]d.ip_cidr_range}"
}

`
	var output string
	for i := range zoneList {
		output += fmt.Sprintf(tmpl, i+1, (i+1)*16)
	}
	return output
}

func readTemplates() templates {
	tmpls := templates{}
	tmpls.vars = string(MustAsset("templates/vars.tf"))
	tmpls.jumpbox = string(MustAsset("templates/jumpbox.tf"))
	tmpls.boshDirector = string(MustAsset("templates/bosh_director.tf"))
	tmpls.cfLB = string(MustAsset("templates/cf_lb.tf"))
	tmpls.cfDNS = string(MustAsset("templates/cf_dns.tf"))
	tmpls.concourseLB = string(MustAsset("templates/concourse_lb.tf"))

	return tmpls
}
