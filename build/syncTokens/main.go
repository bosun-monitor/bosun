package main

import (
	"flag"
	"github.com/garyburd/redigo/redis"
	"log"
)

var (
	src = flag.String("src", "", "Source redis server")
	dst = flag.String("dst", "", "Destination redis server")

	key = flag.String("key", "accessTokens", "Redis Key to sync")
)

func main() {
	flag.Parse()
	if *src == "" || *dst == "" {
		log.Fatal("Both src and dst redis servers required")
	}

	srcC, err := redis.Dial("tcp", *src)
	if err != nil {
		log.Fatal("Dialing src: ", err)
	}
	defer srcC.Close()
	dstC, err := redis.Dial("tcp", *dst)
	if err != nil {
		log.Fatal("Dialing dst: ", err)
	}
	defer dstC.Close()

	ser, err := redis.String(srcC.Do("DUMP", *key))
	if err != nil {
		log.Fatal("Dumping key: ", err)
	}

	_, err = dstC.Do("DEL", *key)
	if err != nil {
		log.Fatal("Deleting at destination: ", err)
	}

	_, err = dstC.Do("RESTORE", *key, 0, ser)
	if err != nil {
		log.Fatal("Restoring: ", err)
	}

	log.Println("Complete")
}
