// Copyright 2018 Synadia

package test

import (
	"testing"
)

func TestCreateClientConnSubscribeAndPublish(t *testing.T) {
	s := RunServer()
	defer s.Shutdown()

	CreateClientConnSubscribeAndPublish(t).Close()
}
