// Allow the service and its sidecar proxy to register into the catalog.
service "service-a" {
    policy = "write"
}
service "service-a-sidecar-proxy" {
    policy = "write"
}

// Allow for any potential upstreams to be resolved.
service_prefix "" {
    policy = "read"
}
node_prefix "" {
    policy = "read"
}
