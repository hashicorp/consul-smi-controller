resource "digitalocean_kubernetes_cluster" "" {
  name    = "foo"
  region  = "nyc1"
  version = "1.12.1-do.2"

  node_pool {
    name       = "worker-pool"
    size       = "s-2vcpu-2gb"
    node_count = 1
  }
}