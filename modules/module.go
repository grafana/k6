package modules

import (
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/master"
)

type Module interface {
	StartClient(client *client.Client)
	StartMaster(master *master.Master)
}
