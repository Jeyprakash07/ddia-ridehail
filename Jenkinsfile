// CD for ingest: deploys every commit on main once CI has published its image.
pipeline {
  agent any

  triggers {
    // GitHub cannot reach this laptop to send a webhook, so Jenkins polls.
    pollSCM('H/2 * * * *')
  }

  options {
    disableConcurrentBuilds()
    timeout(time: 20, unit: 'MINUTES')
  }

  environment {
    IMAGE = "ghcr.io/jeyprakash07/ddia-ridehail:${env.GIT_COMMIT}"
  }

  stages {
    stage('Wait for CI image') {
      steps {
        sh 'bash platform/jenkins/wait-for-image.sh "$GIT_COMMIT"'
      }
    }

    stage('Deploy') {
      steps {
        script { env.DEPLOYED = 'true' }
        sh 'sed "s|ridehail:dev|$IMAGE|g" deploy/ingest.yaml | kubectl apply -f -'
      }
    }

    stage('Verify rollout') {
      steps {
        sh 'kubectl -n ridehail rollout status deployment/ingest --timeout=180s'
      }
    }
  }

  post {
    failure {
      script {
        // Roll back only if this build changed the deployment.
        if (env.DEPLOYED == 'true') {
          sh 'kubectl -n ridehail rollout undo deployment/ingest'
        }
      }
    }
  }
}
