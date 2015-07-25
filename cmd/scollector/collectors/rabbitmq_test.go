package collectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testRabbitURL = "http://guest:guest@localhost:15672"

func TestRabbitmqOverview(t *testing.T) {
	md, err := rabbitmqOverview(testRabbitURL)
	assert.NotNil(t, md)
	assert.Nil(t, err)
}
func TestRabbitmqNodes(t *testing.T) {
	md, err := rabbitmqNodes(testRabbitURL)
	assert.NotNil(t, md)
	assert.Nil(t, err)
}

func TestRabbitmqQueues(t *testing.T) {
	md, err := rabbitmqQueues(testRabbitURL)
	assert.NotNil(t, md)
	assert.Nil(t, err)

}
