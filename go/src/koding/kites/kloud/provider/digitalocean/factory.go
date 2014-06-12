package digitalocean

import (
	"koding/kites/kloud/kloud/protocol"
	"strconv"
	"time"
)

type DoFactory struct {
	client *Client
}

// CreateCacheDroplet creates a new droplet with a key, after creating the
// machine one needs to rename the machine to use it.
func (d *DoFactory) Create() (*Droplet, error) {
	image, err := d.client.Image(protocol.DefaultImageName)
	if err != nil {
		return nil, err
	}

	dropletName := "koding-cache-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	droplet, err := d.client.DropletWithKey(dropletName, image.Id)
	if err != nil {
		return nil, err
	}

	return droplet, nil
}

// Destroy destroys the droplet with the given dropletId
func (d *DoFactory) Destroy(dropletId uint) error {
	_, err := d.client.DestroyDroplet(dropletId)
	return err
}
