package session

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PlayerSkinHandler handles the PlayerSkin packet.
type PlayerSkinHandler struct{}

// Handle ...
func (b PlayerSkinHandler) Handle(p packet.Packet, s *Session) error {
	pk := p.(*packet.PlayerSkin)
	s.c.SetSkin(protocolToSkin(pk.Skin))
	return nil
}
