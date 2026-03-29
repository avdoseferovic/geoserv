package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strings"

	"github.com/avdoseferovic/geoserv/internal/protocol"
	"github.com/ethanmoffat/eolib-go/v3/data"
	eoproto "github.com/ethanmoffat/eolib-go/v3/protocol"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/client"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
)

type TestBot struct {
	id     int
	bus    *protocol.PacketBus
	logger *slog.Logger

	playerID    int
	initialized bool
	mapID       int
	x, y        int
	direction   int
	walkTS      int
	cmdCh       chan string
}

func NewTestBot(id int, addr string) (*TestBot, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	return &TestBot{
		id:     id,
		bus:    protocol.NewPacketBus(protocol.NewTCPConn(conn)),
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		cmdCh:  make(chan string, 8),
	}, nil
}

func (b *TestBot) Run(ctx context.Context) error {
	b.logger.Info("bot started", "id", b.id)

	if err := b.bus.SendPacket(&client.InitInitClientPacket{
		Challenge: rand.Intn(240) + 1,
		Version:   eonet.Version{Major: 0, Minor: 0, Patch: 28},
		Hdid:      fmt.Sprintf("testbot_%d", b.id),
	}); err != nil {
		return fmt.Errorf("send init: %w", err)
	}

	go b.readStdin()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("bot stopped", "id", b.id)
			return nil
		case cmd := <-b.cmdCh:
			if err := b.handleCommand(cmd); err != nil {
				return fmt.Errorf("command %q: %w", cmd, err)
			}
		default:
			if err := b.readPacket(); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				if isTerminalError(err) {
					b.logger.Info("connection closed", "id", b.id, "err", err)
					return nil
				}
				if isTimeoutError(err) {
					continue
				}
				b.logger.Debug("read error", "id", b.id, "err", err)
			}
		}
	}
}

func (b *TestBot) readStdin() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		b.cmdCh <- strings.TrimSpace(scanner.Text())
	}
}

func (b *TestBot) handleCommand(cmd string) error {
	if !b.initialized {
		b.logger.Warn("not in game yet")
		return nil
	}

	switch cmd {
	case "w", "walk":
		return b.doWalk(rand.Intn(4))
	case "wu", "up":
		return b.doWalk(1)
	case "wd", "down":
		return b.doWalk(0)
	case "wl", "left":
		return b.doWalk(2)
	case "wr", "right":
		return b.doWalk(3)
	case "pos":
		b.logger.Info("position", "map", b.mapID, "x", b.x, "y", b.y, "dir", b.direction)
	default:
		b.logger.Info("commands: walk/w, up/wu, down/wd, left/wl, right/wr, pos")
	}
	return nil
}

// sendSequenced sends a packet with a sequence byte prepended to the payload.
func (b *TestBot) sendSequenced(pkt eonet.Packet) error {
	writer := data.NewEoWriter()
	if err := pkt.Serialize(writer); err != nil {
		return fmt.Errorf("serialize: %w", err)
	}

	seqBytes := data.EncodeNumber(b.bus.Sequencer.NextSequence())

	payload := make([]byte, 0, 1+len(writer.Array()))
	payload = append(payload, seqBytes[0])
	payload = append(payload, writer.Array()...)

	return b.bus.Send(pkt.Action(), pkt.Family(), payload)
}

func (b *TestBot) readPacket() error {
	action, family, reader, err := b.bus.Recv()
	if err != nil {
		return err
	}

	b.logger.Debug("packet received", "id", b.id, "family", family, "action", action)

	switch family {
	case eonet.PacketFamily_Init:
		return b.handleInit(action, reader)
	case eonet.PacketFamily_Connection:
		return b.handleConnection(action, reader)
	case eonet.PacketFamily_Login:
		return b.handleLogin(action, reader)
	case eonet.PacketFamily_Character:
		return b.handleCharacter(action, reader)
	case eonet.PacketFamily_Welcome:
		return b.handleWelcome(action, reader)
	case eonet.PacketFamily_Talk:
		return b.handleTalk(action, reader)
	}
	return nil
}

func (b *TestBot) handleInit(action eonet.PacketAction, reader *data.EoReader) error {
	if action != eonet.PacketAction_Init {
		return nil
	}

	var reply server.InitInitServerPacket
	if err := reply.Deserialize(reader); err != nil {
		return fmt.Errorf("deserialize init reply: %w", err)
	}
	if reply.ReplyCode != server.InitReply_Ok {
		return fmt.Errorf("init rejected: code=%d", reply.ReplyCode)
	}

	okData := reply.ReplyCodeData.(*server.InitInitReplyCodeDataOk)
	b.playerID = okData.PlayerId
	b.bus.ServerEncryptionMultiple = okData.ClientEncryptionMultiple
	b.bus.ClientEncryptionMultiple = okData.ServerEncryptionMultiple

	seqStart := okData.Seq1*7 + okData.Seq2 - 13
	b.bus.Sequencer.Reset(seqStart)
	b.bus.Sequencer.NextSequence()

	b.logger.Info("init complete", "id", b.id, "playerID", b.playerID)

	if err := b.sendSequenced(&client.ConnectionAcceptClientPacket{
		PlayerId:                 b.playerID,
		ClientEncryptionMultiple: okData.ClientEncryptionMultiple,
		ServerEncryptionMultiple: okData.ServerEncryptionMultiple,
	}); err != nil {
		return err
	}

	return b.sendSequenced(&client.LoginRequestClientPacket{
		Username: fmt.Sprintf("testbot%d", b.id),
		Password: "testpass",
	})
}

func (b *TestBot) handleConnection(action eonet.PacketAction, reader *data.EoReader) error {
	if action != eonet.PacketAction_Player {
		return nil
	}
	var pkt server.ConnectionPlayerServerPacket
	if err := pkt.Deserialize(reader); err != nil {
		return fmt.Errorf("deserialize ping: %w", err)
	}
	b.bus.Sequencer.SetStart(pkt.Seq1 - pkt.Seq2)
	return b.sendSequenced(&client.ConnectionPingClientPacket{})
}

func (b *TestBot) handleLogin(action eonet.PacketAction, reader *data.EoReader) error {
	var reply server.LoginReplyServerPacket
	if err := reply.Deserialize(reader); err != nil {
		return err
	}
	if reply.ReplyCode != server.LoginReply_Ok {
		return fmt.Errorf("login failed: code=%d", reply.ReplyCode)
	}

	okData := reply.ReplyCodeData.(*server.LoginReplyReplyCodeDataOk)
	b.logger.Info("authenticated", "id", b.id, "characters", len(okData.Characters))

	if len(okData.Characters) > 0 {
		charID := okData.Characters[0].Id
		b.logger.Info("selecting character", "id", b.id, "charID", charID)
		return b.sendSequenced(&client.WelcomeRequestClientPacket{CharacterId: charID})
	}

	b.logger.Info("no characters, creating one", "id", b.id)
	return b.sendSequenced(&client.CharacterCreateClientPacket{
		Name:      fmt.Sprintf("bot%d", b.id),
		Gender:    eoproto.Gender_Female,
		HairStyle: 1,
		HairColor: 1,
		Skin:      1,
	})
}

func (b *TestBot) handleCharacter(action eonet.PacketAction, reader *data.EoReader) error {
	if action != eonet.PacketAction_Reply {
		return nil
	}
	var reply server.CharacterReplyServerPacket
	if err := reply.Deserialize(reader); err != nil {
		return fmt.Errorf("deserialize character reply: %w", err)
	}
	okData, ok := reply.ReplyCodeData.(*server.CharacterReplyReplyCodeDataOk)
	if !ok || len(okData.Characters) == 0 {
		return fmt.Errorf("character created but no characters in reply")
	}
	charID := okData.Characters[0].Id
	b.logger.Info("character created, selecting", "id", b.id, "charID", charID)
	return b.sendSequenced(&client.WelcomeRequestClientPacket{CharacterId: charID})
}

func (b *TestBot) handleWelcome(action eonet.PacketAction, reader *data.EoReader) error {
	if action != eonet.PacketAction_Reply {
		return nil
	}
	var reply server.WelcomeReplyServerPacket
	if err := reply.Deserialize(reader); err != nil {
		return err
	}

	if reply.WelcomeCode == server.WelcomeCode_SelectCharacter {
		d := reply.WelcomeCodeData.(*server.WelcomeReplyWelcomeCodeDataSelectCharacter)
		b.mapID = int(d.MapId)
		b.initialized = true
		b.logger.Info("entering game", "id", b.id, "map", b.mapID)
		return b.sendSequenced(&client.WelcomeMsgClientPacket{
			SessionId:   d.SessionId,
			CharacterId: d.CharacterId,
		})
	}

	if reply.WelcomeCode == server.WelcomeCode_EnterGame {
		d := reply.WelcomeCodeData.(*server.WelcomeReplyWelcomeCodeDataEnterGame)
		// Read initial position from nearby characters
		for _, c := range d.Nearby.Characters {
			if c.PlayerId == b.playerID {
				b.x = c.Coords.X
				b.y = c.Coords.Y
				b.direction = int(c.Direction)
				break
			}
		}
		b.logger.Info("in game", "id", b.id, "map", b.mapID, "x", b.x, "y", b.y)
		fmt.Println("Type a command: walk, up, down, left, right, pos")
	}

	return nil
}

const owner = "avdo"

func (b *TestBot) handleTalk(action eonet.PacketAction, reader *data.EoReader) error {
	// Listen for private messages from owner
	if action != eonet.PacketAction_Tell {
		return nil
	}
	var pkt server.TalkTellServerPacket
	if err := pkt.Deserialize(reader); err != nil {
		return nil
	}
	if !strings.EqualFold(pkt.PlayerName, owner) {
		return nil
	}

	cmd := strings.TrimSpace(strings.ToLower(pkt.Message))
	b.logger.Info("owner command", "from", pkt.PlayerName, "cmd", cmd)

	if !b.initialized {
		return nil
	}

	switch cmd {
	case "walk", "w":
		return b.doWalk(rand.Intn(4))
	case "up", "wu":
		return b.doWalk(1)
	case "down", "wd":
		return b.doWalk(0)
	case "left", "wl":
		return b.doWalk(2)
	case "right", "wr":
		return b.doWalk(3)
	case "pos":
		b.logger.Info("position", "map", b.mapID, "x", b.x, "y", b.y, "dir", b.direction)
	}
	return nil
}

func (b *TestBot) doWalk(dir int) error {
	newX, newY := b.x, b.y
	switch dir {
	case 0:
		newY++
	case 1:
		newY--
	case 2:
		newX--
	case 3:
		newX++
	}
	b.x, b.y = max(1, min(50, newX)), max(1, min(50, newY))
	b.direction = dir
	b.walkTS++

	b.logger.Debug("walking", "dir", dir, "x", b.x, "y", b.y)
	return b.sendSequenced(&client.WalkPlayerClientPacket{
		WalkAction: client.WalkAction{
			Direction: eoproto.Direction(dir),
			Timestamp: b.walkTS,
			Coords:    eoproto.Coords{X: b.x, Y: b.y},
		},
	})
}

func isTerminalError(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: testbot <server_address>")
		fmt.Println("Example: testbot localhost:8078")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bot, err := NewTestBot(1, os.Args[1])
	if err != nil {
		slog.Error("failed to create bot", "err", err)
		os.Exit(1)
	}

	if err := bot.Run(ctx); err != nil {
		slog.Error("bot error", "err", err)
		os.Exit(1)
	}
}
