package servers

import (
	"context"
	"crypto/rand"
	_ "embed"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/cfoust/sour/pkg/assets"
	"github.com/cfoust/sour/pkg/config"
	C "github.com/cfoust/sour/pkg/game/constants"
	P "github.com/cfoust/sour/pkg/game/protocol"
	"github.com/cfoust/sour/pkg/gameserver"
	"github.com/cfoust/sour/pkg/maps"
	"github.com/cfoust/sour/pkg/server/ingress"

	"github.com/repeale/fp-go"
	"github.com/repeale/fp-go/option"
	"github.com/rs/zerolog/log"
)

// Sauerbraten servers assign each client a number.
// NOT the same thing as ClientID.
type ClientNum int32

const (
	// How long we wait before pruning an unused server
	SERVER_MAX_IDLE_TIME = time.Duration(10 * time.Minute)
	// Private lobby codes stop resolving shortly after the last player leaves.
	PRIVATE_LOBBY_IDLE_TIME = 30 * time.Second
)

type MapRequest struct {
	Map  string
	Mode int32
}

type ClientPacket struct {
	Client   ingress.ClientID
	Channel  uint8
	Messages []P.Message
	Server   *GameServer
}

type ClientKick struct {
	Client ingress.ClientID
	Reason int32
	Text   string
}

type ClientLeave struct {
	Client ingress.ClientID
	Num    ClientNum
}

type ClientName struct {
	Client ingress.ClientID
	Name   string
}

type ServerManager struct {
	Servers []*GameServer
	Receive chan []byte
	Mutex   sync.Mutex
	identities map[string]struct{}

	presets []config.Preset
	Maps    *assets.AssetFetcher

	serverDescription string

	kicks   chan ClientKick
	packets chan ClientPacket
}

func (manager *ServerManager) ReceivePackets() <-chan ClientPacket {
	return manager.packets
}

func (manager *ServerManager) ReceiveKicks() <-chan ClientKick {
	return manager.kicks
}

func (manager *ServerManager) GetServerInfo() *ServerInfo {
	info := ServerInfo{}

	manager.Mutex.Lock()
	for _, server := range manager.Servers {
		serverInfo := server.GetServerInfo()
		info.NumClients += serverInfo.NumClients
	}
	manager.Mutex.Unlock()

	return &info
}

func NewServerManager(
	maps *assets.AssetFetcher,
	serverDescription string,
	presets []config.Preset,
) *ServerManager {
	return &ServerManager{
		Servers:           make([]*GameServer, 0),
		identities:        make(map[string]struct{}),
		Maps:              maps,
		serverDescription: serverDescription,
		presets:           presets,
		kicks:             make(chan ClientKick, 100),
		packets:           make(chan ClientPacket, 100),
	}
}

func (manager *ServerManager) Start() error {
	return nil
}

func (manager *ServerManager) Shutdown() {
	for _, server := range manager.Servers {
		server.Shutdown()
	}
}

// TODO this is bad, server identifiers can collide
func FindIdentity() string {
	const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	const codeLength = 5
	code := make([]byte, codeLength)
	max := big.NewInt(int64(len(alphabet)))
	for i := range code {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(fmt.Errorf("generate private lobby code: %w", err))
		}
		code[i] = alphabet[index.Int64()]
	}
	return string(code)
}

func (manager *ServerManager) nextIdentity() string {
	for {
		identity := FindIdentity()
		manager.Mutex.Lock()
		if manager.identities == nil {
			manager.identities = make(map[string]struct{})
		}
		_, exists := manager.identities[identity]
		if !exists {
			manager.identities[identity] = struct{}{}
		}
		manager.Mutex.Unlock()
		if !exists {
			return identity
		}
	}
}

func (manager *ServerManager) RemoveServer(server *GameServer) {
	server.Shutdown()

	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()

	manager.Servers = fp.Filter(func(v *GameServer) bool { return v.Id != server.Id })(manager.Servers)
	delete(manager.identities, server.Id)
}

func (manager *ServerManager) PruneServers(ctx context.Context) {
	interval := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-interval.C:
			manager.Mutex.Lock()

			toPrune := make([]*GameServer, 0)

			for _, server := range manager.Servers {
				server.Mutex.RLock()
				lastEvent := server.LastEvent
				numClients := server.NumClients()
				server.Mutex.RUnlock()
				idleLimit := SERVER_MAX_IDLE_TIME
				if server.Private {
					idleLimit = PRIVATE_LOBBY_IDLE_TIME
				}
				if (time.Now().Sub(lastEvent)) < idleLimit || numClients > 0 || server.Alias != "" {
					continue
				}
				toPrune = append(toPrune, server)
			}

			manager.Mutex.Unlock()

			for _, server := range toPrune {
				logger := server.Logger()
				logger.Info().Msg("server was pruned")
				server.Cancel()
			}

			continue
		case <-ctx.Done():
			return
		}
	}
}

func (manager *ServerManager) ReadEntities(ctx context.Context, server *GameServer, data []byte) error {
	map_, err := maps.BasicsFromGZ(data)
	if err != nil {
		log.Error().Err(err).Msgf("could not read map entities")
		return err
	}

	server.Mutex.Lock()
	server.Entities = map_.Entities
	server.Mutex.Unlock()
	return nil
}

func (manager *ServerManager) PollMapRequests(ctx context.Context, server *GameServer) {
	requests := server.ReceiveMaps()

	for {
		select {
		case request := <-requests:
			logger := log.With().Str("map", request).Logger()

			if request == "" {
				continue
			}

			data, err := manager.Maps.FetchMapBytes(ctx, request)
			if err != nil {
				logger.Error().Err(err).Msg("failed to download map")
				continue
			}

			go manager.ReadEntities(ctx, server, data)
		case <-ctx.Done():
			return
		}
	}
}

func (manager *ServerManager) FindPreset(presetName string, isVirtualOk bool) opt.Option[config.Preset] {
	for _, preset := range manager.presets {
		if (preset.Name == presetName || (len(presetName) == 0 && preset.Default)) && (isVirtualOk || !preset.Virtual) {
			return opt.Some(preset)
		}
	}

	return opt.None[config.Preset]()
}

func (manager *ServerManager) NewServer(ctx context.Context, presetName string, isVirtualOk bool, private ...bool) (*GameServer, error) {
	found := manager.FindPreset(presetName, isVirtualOk)

	if opt.IsNone(found) {
		return nil, fmt.Errorf("failed to find server preset %s and there is no default", presetName)
	}

	config := found.Value.Config

	privateLobby := len(private) > 0 && private[0]
	server := GameServer{
		Server:    gameserver.New(ctx, &config),
		Alias:     "",
		LastEvent: time.Now(),
		Entities:  make([]maps.Entity, 0),
		Hidden:    false,
		Private:   privateLobby,
		Started:   time.Now(),
		kicks:     manager.kicks,
		packets:   manager.packets,
		Id:        manager.nextIdentity(),

		From: P.NewMessageProxy(false),
		To:   P.NewMessageProxy(true),
	}

	server.SetDescription(
		strings.ReplaceAll(manager.serverDescription, "#id", server.Id),
	)

	mode := C.GetModeNumber(config.DefaultMode)
	if opt.IsNone(mode) {
		manager.Mutex.Lock()
		delete(manager.identities, server.Id)
		manager.Mutex.Unlock()
		return nil, fmt.Errorf("invalid mode name: %s", config.DefaultMode)
	}

	server.ChangeMap(int32(mode.Value), config.DefaultMap)

	go server.Poll(server.Ctx())
	go manager.PollMapRequests(server.Ctx(), &server)

	go func() {
		for {
			select {
			case packet := <-server.Outgoing():
				manager.packets <- ClientPacket{
					Client:   ingress.ClientID(packet.Session),
					Channel:  packet.Channel,
					Messages: packet.Messages,
					Server:   &server,
				}
			case <-server.Ctx().Done():
				return
			}
		}
	}()

	manager.Mutex.Lock()
	manager.Servers = append(manager.Servers, &server)
	manager.Mutex.Unlock()

	// Remove the server when it exits for any reason
	go func() {
		<-server.Ctx().Done()
		manager.RemoveServer(&server)
	}()

	return &server, nil
}
