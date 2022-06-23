package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	ver "github.com/hashicorp/go-version"
)

type LastVersionEngines struct {
	Redis     *ver.Version
	Memcached *ver.Version
}

type Cluster struct {
	name                     *string
	arn                      *string
	tags                     *[]types.Tag
	clusterInstanceType      *string
	clusterType              *string
	transitEncryptionEnabled *bool
	atRestEncrytionEnabled   *bool
	engineActualVersion      *string
	engineLastVersion        *string
}

func updatedLastVersionEngines(client *elasticache.Client) LastVersionEngines {
	lastVersionEngines := LastVersionEngines{}
	var err error
	lastVersionEngines.Redis, err = ver.NewVersion("0.0.0")
	if err != nil {
		log.Fatal(err)
	}
	lastVersionEngines.Memcached, _ = ver.NewVersion("0.0.0")
	paginator := elasticache.NewDescribeCacheEngineVersionsPaginator(client, &elasticache.DescribeCacheEngineVersionsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		for _, version := range page.CacheEngineVersions {
			localVersion, err := ver.NewVersion(*version.EngineVersion)
			if err != nil {
				log.Fatal(err)
			}
			if *version.Engine == "redis" {
				if lastVersionEngines.Redis.LessThanOrEqual(localVersion) {
					lastVersionEngines.Redis = localVersion
				}
			} else if *version.Engine == "memcached" {
				if lastVersionEngines.Memcached.LessThanOrEqual(localVersion) {
					lastVersionEngines.Memcached = localVersion
				}
			} else {
				log.Fatal("Some engine not in \"memcached\" or \"redis\": ", *version.Engine)
			}
			ver.NewVersion(*version.CacheEngineVersionDescription)

		}
	}
	return lastVersionEngines
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("Error getting configuration from ~/.aws/*: ", err)
	}

	if len(os.Args) == 2 {
		cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(os.Args[1]))
		if err != nil {
			log.Fatal("Error getting configuration from ~/.aws/*: ", err)
		}
	} else if len(os.Args) > 2 {
		log.Fatal("Usage: elastic-cluster-info [ <aws profile> ] \n \t With <aws profile> a previously set aws credential profile stored in ~/.aws/* \n \t If no profile given, it picks the default one.")
	}

	client := elasticache.NewFromConfig(cfg)

	lastVersionEngines := updatedLastVersionEngines(client)
	clusters := []Cluster{}

	paginator := elasticache.NewDescribeCacheClustersPaginator(client, &elasticache.DescribeCacheClustersInput{})

	var alreadyTaken = map[string]bool{}

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		for _, elasticachecluster := range page.CacheClusters {

			cluster := Cluster{
				name:                     elasticachecluster.CacheClusterId,
				arn:                      elasticachecluster.ARN,
				atRestEncrytionEnabled:   elasticachecluster.AtRestEncryptionEnabled,
				transitEncryptionEnabled: elasticachecluster.TransitEncryptionEnabled,
				clusterInstanceType:      elasticachecluster.CacheNodeType,
				engineActualVersion:      elasticachecluster.EngineVersion,
				clusterType:              elasticachecluster.Engine,
			}
			if *cluster.clusterType == "redis" {
				cluster.engineLastVersion = new(string)
				*cluster.engineLastVersion = lastVersionEngines.Redis.Original()
			} else {
				cluster.engineLastVersion = new(string)
				*cluster.engineLastVersion = lastVersionEngines.Memcached.Original()
			}
			tags, err := client.ListTagsForResource(context.TODO(), &elasticache.ListTagsForResourceInput{ResourceName: cluster.arn})

			if err != nil {
				log.Fatal("Error getting tags from resource: ", *cluster.arn)
			}
			cluster.tags = &tags.TagList

			if elasticachecluster.ReplicationGroupId != nil {
				if !alreadyTaken[*elasticachecluster.ReplicationGroupId] {
					cluster.name = elasticachecluster.ReplicationGroupId
					alreadyTaken[*elasticachecluster.ReplicationGroupId] = true
					clusters = append(clusters, cluster)
				}
			} else {
				clusters = append(clusters, cluster)
			}
		}
	}

	csvArray := [][]string{
		{"ClusterID", "ARN", "InstanceType", "ClusterType", "CurrentEngineVersion", "LatestEngineVersion", "Tags", "EncryptionAtRestEnabled", "EncryptionAtTransitEnabled"},
	}

	for _, cluster := range clusters {
		tags := ""
		for _, tag := range *cluster.tags {
			printabletag, _ := json.Marshal(tag)
			log.Print(string(printabletag))
			tags += fmt.Sprintf("|%v: %v|", *tag.Key, *tag.Value)
		}
		csvArray = append(csvArray, []string{*cluster.name, *cluster.arn, *cluster.clusterInstanceType, *cluster.clusterType, *cluster.engineActualVersion, *cluster.engineLastVersion, tags, strconv.FormatBool(*cluster.atRestEncrytionEnabled), strconv.FormatBool(*cluster.transitEncryptionEnabled)})
	}
	csvFile, err := os.Create(cfg.Region + ".csv")

	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	defer csvFile.Close()

	w := csv.NewWriter(csvFile)
	w.WriteAll(csvArray)
	if err := w.Error(); err != nil {
		log.Fatalln("error writing csv:", err)
	}
}
