# Jenkins plus kubectl, for deploying to the local kind cluster.
FROM jenkins/jenkins:lts-jdk21

USER root
ARG KUBECTL_VERSION=v1.37.2
RUN curl -fsSLo /usr/local/bin/kubectl \
      "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
 && chmod 0755 /usr/local/bin/kubectl
USER jenkins
