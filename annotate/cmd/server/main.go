package main

import (
	"flag"
	"log"
	"net/http"

	"bosun.org/annotate/backend"
	"bosun.org/annotate/web"
	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
)

type Conf struct {
	ListenAddress   string
	ElasticClusters []ElasticCluster
}

type ElasticCluster struct {
	Servers []string // i.w. http://ny-elastic01:9200
	Index   string
}

var confFlag = flag.String("conf", "config.toml", "config file")
var localFlag = flag.Bool("dev", false, "use local assets, must be run from annotate/web dir.")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	var c Conf
	if _, err := toml.DecodeFile(*confFlag, &c); err != nil {
		log.Fatal("failed to decode config file: ", err)
	}
	backends := []backend.Backend{}
	for _, eCluster := range c.ElasticClusters {
		b, err := backend.NewElastic(eCluster.Servers, eCluster.Index)
		if err != nil {
			log.Fatal(err)
		}
		backends = append(backends, b)
	}
	for _, b := range backends {
		if err := b.InitBackend(); err != nil {
			log.Fatal(err)
		}
	}
	router := mux.Router{}
	if err := web.AddRoutes(&router, "", backends, true, *localFlag); err != nil {
		log.Fatal(err)
	}
	http.Handle("/", &router)
	go func() {
		log.Fatal(http.ListenAndServe(c.ListenAddress, nil))
	}()
	select {}
}

// Reference Conf
//ListenAddress = ":8080"

//[[ElasticClusters]]
//Servers = [ "http://ny-devlogstash04:9200" ]
//Index = "annotate"
