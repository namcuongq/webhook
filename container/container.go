package container

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/mgo.v2"
)

type Config struct {
	Listen string
	User   string
	Pass   string

	//mongo
	MongoDatabase       string
	MongoServer         []string
	MongoReplicaSetName string
	MongoUser           string
	MongoPassword       string
	MongoAuthDB         string
}

type Contanier struct {
	Config *Config
	DB     *mgo.Database
}

var (
	container *Contanier
)

func Get() *Contanier {
	return container
}

func Setup(pathConfig string) error {
	container = new(Contanier)
	container.Config = new(Config)

	if _, err := toml.DecodeFile(pathConfig, container.Config); err != nil {
		return fmt.Errorf("Could not load config: %v", err)
	}

	err := container.loadDB()
	if err != nil {
		return err
	}

	return nil
}

func (container *Contanier) loadDB() (err error) {
	info := &mgo.DialInfo{
		Addrs:          container.Config.MongoServer,
		Timeout:        30 * time.Second,
		ReplicaSetName: container.Config.MongoReplicaSetName,
		Username:       container.Config.MongoUser,
		Password:       container.Config.MongoPassword,
		Database:       container.Config.MongoAuthDB,
	}

	sesstion, err := mgo.DialWithInfo(info)
	if err != nil {
		return
	}

	container.DB = sesstion.DB(container.Config.MongoDatabase)
	container.DB.Session.SetSocketTimeout(10 * time.Minute)
	return
}
