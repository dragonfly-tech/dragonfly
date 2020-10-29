package session

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// AdventureSettingsHandler handles the AdventureSettings packet.
type AdventureSettingsHandler struct{}

// Handle ...
func (b AdventureSettingsHandler) Handle(p packet.Packet, s *Session) error {
	pk := p.(*packet.AdventureSettings)
	switch {
	case pk.Flags&packet.AdventureFlagFlying != 0:
		if !s.c.CanFly() {
			return nil // Client cannot fly because flight is disabled.
		}
	case pk.Flags&packet.AdventureFlagAllowFlight != 0:
		if s.adventureFlags.Load()&packet.AdventureFlagAllowFlight == 0 {
			return nil // Client is trying to allow flight for itself.
		}
	case pk.Flags&packet.AdventureFlagNoClip != 0:
		if s.adventureFlags.Load()&packet.AdventureFlagNoClip == 0 {
			return nil // Client is trying to noclip.
		}
	case pk.Flags&packet.AdventureFlagWorldBuilder != 0:
		if s.adventureFlags.Load()&packet.AdventureFlagWorldBuilder == 0 {
			return nil // Client is trying to enable WorldBuilder flag
		}
	case pk.Flags&packet.AdventureFlagNoPVP != 0:
		if s.adventureFlags.Load()&packet.AdventureFlagNoPVP == 0 {
			return nil // Client is trying to enable NoPVP flag
		}
	case pk.Flags&packet.AdventureFlagWorldImmutable != 0:
		if s.adventureFlags.Load()&packet.AdventureFlagWorldImmutable == 0 {
			return nil // Client is trying to enable WorldImmutable flag
		}
	}
	s.adventureFlags.Store(pk.Flags)
	return nil
}
