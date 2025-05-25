#!/usr/bin/env bash
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

create_test_users() {
  echo CREATING USER PASSWORDS SECRET
  htpasswd -c -B -b users.htpasswd admin admin
  htpasswd -B -b users.htpasswd user1 user1
  htpasswd -B -b users.htpasswd user2 user2

  oc create ns openshift-config
  oc create secret generic htpass-user-test --from-file=htpasswd=users.htpasswd -n openshift-config
  rm -f users.htpasswd
}

create_auth_provider() {
  echo CREATING AUTH PROVIDER
  cat >oauth.yaml <<EOL
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
    name: cluster
spec:
    identityProviders:
        - name: users
          mappingMethod: claim
          type: HTPasswd
          htpasswd:
              fileData:
                  name: htpass-user-test
EOL
  oc apply -f oauth.yaml
  rm -f oauth.yaml
}

create_role_bindings() {
  echo CREATING ROLE BINDINGS
  oc create clusterrolebinding cluster-manager-admin-binding --clusterrole=open-cluster-management:cluster-manager-admin --user=admin
  oc create rolebinding view-binding-user1 --clusterrole=view --user=user1 -n local-cluster
}

if ! which htpasswd &>/dev/null; then
  if which yum &>/dev/null; then
    sudo yum update
    sudo yum install -y httpd-tools
  else
    echo "Error: Package manager yum not found. Failed to find or install htpasswd."
    exit 1
  fi
fi

clean() {
  echo "CLEANING OLD RBAC SETTINGS"
  oc delete clusterrolebinding cluster-manager-admin-binding &>/dev/null
  oc delete rolebinding view-binding-user1 -n local-cluster &>/dev/null

  oc delete identity users:user1 &>/dev/null
  oc delete identity users:user2 &>/dev/null
  oc delete identity users:admin &>/dev/null
  oc delete user admin &>/dev/null
  oc delete user user1 &>/dev/null
  oc delete user user2 &>/dev/null
  oc delete secret htpass-user-test -n openshift-config &>/dev/null

  cat >oauth_del.yaml <<EOL
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
    name: cluster
spec: {}
EOL
  oc apply -f oauth_del.yaml
  rm -rf oauth_del.yaml

}

clean
create_test_users
create_auth_provider
create_role_bindings
