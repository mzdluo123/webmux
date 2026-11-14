package api

import (
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/transport"
)

func NewBridge(deps Deps) transport.Bridge {
	server := newServer(deps)
	tcp := server.routes(transport.TCP)
	direct := server.routes(transport.P2P)
	handler := server.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/local/") {
			http.NotFound(w, r)
			return
		}
		tcp.ServeHTTP(w, r)
	}))
	streams := server.streams
	for _, contribution := range server.Contributions {
		for _, stream := range contribution.Streams {
			if stream.Exposure&transport.P2P != 0 {
				streams = append(streams, stream)
			}
		}
	}
	return transport.Bridge{HTTPHandler: handler, ServeStream: transport.StreamRouter(streams), ServeRPC: transport.RPCHandler(direct, server.authorized)}
}

func (s *Server) hasMCP() bool { return s.hasContribution("mcp") }
func (s *Server) hasContribution(owner string) bool {
	for _, contribution := range s.Contributions {
		if contribution.Owner == owner {
			return true
		}
	}
	return false
}
