docker container based on spotify docker image modified for version
update

http://cassandra.apache.org/download/

last tested using github.com/davidwalter0/docker-cassandra

some of the dockerfile based on github.com/spotify/cassandra

remaking this to be a golang <-> cassandra

build and run a single node cassandra container if you don't have an
existing cassandra cluster

```
git clone github.com/davidwalter0/docker-cassandra

pushd docker-cassandra/cassandra-base
docker build --tag davidwalter0/cassandra:base .

popd; pushd docker-cassandra/cassandra
docker build --tag davidwalter0/cassandra .

popd; pushd docker-cassandra
scripts/run-cassandra-container


go run connect.go helper.go 

```


Cassandra investigation

SHOW HOST
SHOW VERSION
SELECT * FROM system.local;
nodetool status



K8s pod name downward api

  env:
    - name: POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
