pluginManagement {
  repositories {
    maven(url = "https://proxy.wn.leyux.de/maven/")
  }
}

buildCache {
  local {
    enabled = false
  }
  // This is the git-pkgs proxy Gradle HTTP Build Cache endpoint from the upstream README.
  // It is not part of the Sonatype Firewall Pro package-blocking path.
  remote<HttpBuildCache> {
    url = uri("https://proxy.wn.leyux.de/gradle/")
    push = false
  }
}
