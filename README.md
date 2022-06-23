# elasticache-cluster-info
Command line utility for getting and printing information about AWS ElasticCache cluster.
You should have a credentials file stored in ~/.aws/credentials and at least a default profile configured, you can pass the profile you want to get clusters information about as parameter to the program.

## installation:
```
git clone this repo
go build
```

## Usage
```
./elastic-cluster-info [ <aws profile> ]
```
<aws profile>	is a previously set aws credential profile stored in ~/.aws/*
				If no profile given, it picks the default one.
