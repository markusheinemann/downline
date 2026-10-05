package shipper

import (
	"net"
	"testing"

	"github.com/pkg/sftp"
)

func newTestSFTPFS(t *testing.T) remoteFS {
	t.Helper()

	serverCon, clientCon := net.Pipe()

	server := sftp.NewRequestServer(serverCon, sftp.InMemHandler())
	go server.Serve()

	client, err := sftp.NewClientPipe(clientCon, clientCon)
	if err != nil {
		t.Fatalf("connect to sftp server: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
		server.Close()
	})

	return &SFTP{client: client}
}

func TestSFTPFS_Contract(t *testing.T) {
	testRemoteFSContract(t, newTestSFTPFS)
}
