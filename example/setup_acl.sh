#! /bin/bash

consul acl policy create -name service-a -rules @acl_rules_service_a.hcl

consul acl role create -name "service-a" \
                       -description "Role for service a" \
                       -policy-name "service-a" \
                       -service-identity "service-a"
