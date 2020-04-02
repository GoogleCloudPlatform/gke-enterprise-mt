output network_name {
  value = module.vpc_shared_net.network_name
}

output subnets_names {
  value = module.vpc_shared_net.subnets_names
}

output secondary_ranges {
  value = { for s in var.subnets :
    s.name => {
      pod = format("%s-pod-secondary", s.name)
      svc = format("%s-services-secondary", s.name)
    }
  }
}
