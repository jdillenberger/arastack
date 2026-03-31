package template

import (
	"testing"
	"gopkg.in/yaml.v3"
)

func TestAppMetaRoutingParsing(t *testing.T) {
	data := []byte(`
name: wireguard
ports:
  - host: 51820
    container: 51820
    protocol: udp
  - host: 51821
    container: 51821
    protocol: tcp
health_check:
  url: "http://localhost:51821"
  interval: 30s
routing:
  container_port: 51821
dependencies:
  - docker
`)
	var meta AppMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Routing == nil {
		t.Fatal("Routing is nil after unmarshal")
	}
	if meta.Routing.ContainerPort != 51821 {
		t.Errorf("ContainerPort = %d, want 51821", meta.Routing.ContainerPort)
	}
}
