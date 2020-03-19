output secondary_ranges {
  value = { for s in var.subnets :
    s.name => {
      pod = format("%s-pod-secondary", s.name)
      svc = format("%s-services-secondary", s.name)
    }
  }
}
