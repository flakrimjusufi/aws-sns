job "${job_name}" {
  datacenters = ["${job_datacenter}"]
  type = "service"
  name = "${job_name}"

  group "instance" {
    count = 1

    network {
      port "http" {
        to = 80
      }
    }

    task "v1" {
      driver = "docker"

      config {
        image = "${job_image}"
        ports = ["http"]
      }

      service {
        name = "${job_name}"
        port = "http"

        tags = [
          "traefik.tags=external",
          "traefik.http.routers.${job_name}.rule=Host(`${route_domain}`) && PathPrefix(`${route_path}`)",
          "traefik.http.routers.${job_name}.tls.certResolver=letsencrypt",
          "traefik.http.routers.${job_name}.middlewares=${job_name}-stripprefix",
          "traefik.http.middlewares.${job_name}-stripprefix.stripprefix.prefixes=${route_path}"
        ]

        check {
          type     = "http"
          path     = "/index.html"
          interval = "10s"
          timeout  = "2s"
        }
      }

      resources {
        cpu    = ${service_cpu_mhz} # MHz
        memory = ${service_memory_mb} # MB
      }

      env = {
        "TESTVARR" = "${env_TESTVARR}"
      }
    }
  }
}
