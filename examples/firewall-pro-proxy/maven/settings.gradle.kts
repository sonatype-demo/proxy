pluginManagement {
  repositories {
    maven(url = "${System.getenv("PROXY_URL") ?: "http://localhost:8080"}/maven/")
  }
}

buildCache {
  local {
    enabled = false
  }
  // This is the git-pkgs proxy Gradle HTTP Build Cache endpoint from the upstream README.
  // It is not part of the Sonatype Firewall Pro package-blocking path.
  remote<HttpBuildCache> {
    url = uri("${System.getenv("PROXY_URL") ?: "http://localhost:8080"}/gradle/")
    push = false
  }
}
