package session

import (
	"encoding/json"
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly/block"
	"github.com/df-mc/dragonfly/dragonfly/entity/effect"
	"github.com/df-mc/dragonfly/dragonfly/internal/entity_internal"
	"github.com/df-mc/dragonfly/dragonfly/internal/nbtconv"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/item/inventory"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/df-mc/dragonfly/dragonfly/player/skin"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/gamemode"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"go.uber.org/atomic"
	"math"
	"net"
	"strings"
	"time"
	_ "unsafe" // Imported for compiler directives.
)

// closeCurrentContainer closes the container the player might currently have open.
func (s *Session) closeCurrentContainer() {
	if !s.containerOpened.Load() {
		return
	}
	s.closeWindow()
	pos := s.openedPos.Load().(world.BlockPos)
	if container, ok := s.c.World().Block(pos).(block.Container); ok {
		container.RemoveViewer(s, s.c.World(), pos)
	}
}

// SendRespawn spawns the controllable of the session client-side in the world, provided it is has died.
func (s *Session) SendRespawn() {
	s.writePacket(&packet.Respawn{
		Position:        vec64To32(s.c.Position().Add(entityOffset(s.c))),
		State:           packet.RespawnStateReadyToSpawn,
		EntityRuntimeID: selfEntityRuntimeID,
	})
}

// sendInv sends the inventory passed to the client with the window ID.
func (s *Session) sendInv(inv *inventory.Inventory, windowID uint32) {
	pk := &packet.InventoryContent{
		WindowID: windowID,
		Content:  make([]protocol.ItemInstance, 0, s.inv.Size()),
	}
	for _, i := range inv.All() {
		pk.Content = append(pk.Content, instanceFromItem(i))
	}
	s.writePacket(pk)
}

const (
	containerArmour         = 6
	containerChest          = 7
	containerBeacon         = 8
	containerFullInventory  = 12
	containerCraftingGrid   = 13
	containerHotbar         = 27
	containerInventory      = 28
	containerOffHand        = 33
	containerCursor         = 58
	containerCreativeOutput = 59
)

// invByID attempts to return an inventory by the ID passed. If found, the inventory is returned and the bool
// returned is true.
func (s *Session) invByID(id int32) (*inventory.Inventory, bool) {
	switch id {
	case containerCraftingGrid, containerCreativeOutput, containerCursor:
		// UI inventory.
		return s.ui, true
	case containerHotbar, containerInventory, containerFullInventory:
		// Hotbar 'inventory', rest of inventory, inventory when container is opened.
		return s.inv, true
	case containerOffHand:
		return s.offHand, true
	case containerArmour:
		// Armour inventory.
		return s.armour.Inv(), true
	case containerChest:
		// Chests, potentially other containers too.
		if s.containerOpened.Load() {
			b := s.c.World().Block(s.openedPos.Load().(world.BlockPos))
			if _, chest := b.(block.Chest); chest {
				return s.openedWindow.Load().(*inventory.Inventory), true
			}
		}
	case containerBeacon:
		if s.containerOpened.Load() {
			b := s.c.World().Block(s.openedPos.Load().(world.BlockPos))
			if _, beacon := b.(block.Beacon); beacon {
				return s.ui, true
			}
		}
	}
	return nil, false
}

// Disconnect disconnects the client and ultimately closes the session. If the message passed is non-empty,
// it will be shown to the client.
func (s *Session) Disconnect(message string) {
	if s != Nop {
		s.writePacket(&packet.Disconnect{
			HideDisconnectionScreen: message == "",
			Message:                 message,
		})
		_ = s.conn.Flush()
	}
}

// SendSpeed sends the speed of the player in an UpdateAttributes packet, so that it is updated client-side.
func (s *Session) SendSpeed(speed float64) {
	s.writePacket(&packet.UpdateAttributes{
		EntityRuntimeID: selfEntityRuntimeID,
		Attributes: []protocol.Attribute{{
			Name:    "minecraft:movement",
			Value:   float32(speed),
			Max:     math.MaxFloat32,
			Min:     0,
			Default: 0.1,
		}},
	})
}

// SendFood ...
func (s *Session) SendFood(food int, saturation, exhaustion float64) {
	s.writePacket(&packet.UpdateAttributes{
		EntityRuntimeID: selfEntityRuntimeID,
		Attributes: []protocol.Attribute{
			{
				Name:  "minecraft:player.hunger",
				Value: float32(food),
				Max:   20, Min: 0, Default: 20,
			},
			{
				Name:  "minecraft:player.saturation",
				Value: float32(saturation),
				Max:   20, Min: 0, Default: 20,
			},
			{
				Name:  "minecraft:player.exhaustion",
				Value: float32(exhaustion),
				Max:   5, Min: 0, Default: 0,
			},
		},
	})
}

// SendVelocity sends the velocity of the player to the client.
func (s *Session) SendVelocity(velocity mgl64.Vec3) {
	s.writePacket(&packet.SetActorMotion{
		EntityRuntimeID: selfEntityRuntimeID,
		Velocity:        vec64To32(velocity),
	})
}

// SendForm sends a form to the client of the connection. The Submit method of the form is called when the
// client submits the form.
func (s *Session) SendForm(f form.Form) {
	var n []map[string]interface{}
	m := map[string]interface{}{}

	switch frm := f.(type) {
	case form.Custom:
		m["type"], m["title"] = "custom_form", frm.Title()
		for _, e := range frm.Elements() {
			n = append(n, elemToMap(e))
		}
		m["content"] = n
	case form.Menu:
		m["type"], m["title"], m["content"] = "form", frm.Title(), frm.Body()
		for _, button := range frm.Buttons() {
			v := map[string]interface{}{"text": button.Text}
			if button.Image != "" {
				buttonType := "path"
				if strings.HasPrefix(button.Image, "http:") || strings.HasPrefix(button.Image, "https:") {
					buttonType = "url"
				}
				v["image"] = map[string]interface{}{"type": buttonType, "data": button.Image}
			}
			n = append(n, v)
		}
		m["buttons"] = n
	case form.Modal:
		m["type"], m["title"], m["content"] = "modal", frm.Title(), frm.Body()
		buttons := frm.Buttons()
		m["button1"], m["button2"] = buttons[0].Text, buttons[1].Text
	}
	b, _ := json.Marshal(m)

	h := s.handlers[packet.IDModalFormResponse].(*ModalFormResponseHandler)
	id := h.currentID.Add(1)

	h.mu.Lock()
	if len(h.forms) > 10 {
		s.log.Debugf("SendForm %v: more than 10 active forms: dropping an existing one.", s.c.Name())
		for k := range h.forms {
			delete(h.forms, k)
			break
		}
	}
	h.forms[id] = f
	h.mu.Unlock()

	s.writePacket(&packet.ModalFormRequest{
		FormID:   id,
		FormData: b,
	})
}

// elemToMap encodes a form element to its representation as a map to be encoded to JSON for the client.
func elemToMap(e form.Element) map[string]interface{} {
	switch element := e.(type) {
	case form.Toggle:
		return map[string]interface{}{
			"type":    "toggle",
			"text":    element.Text,
			"default": element.Default,
		}
	case form.Input:
		return map[string]interface{}{
			"type":        "input",
			"text":        element.Text,
			"default":     element.Default,
			"placeholder": element.Placeholder,
		}
	case form.Label:
		return map[string]interface{}{
			"type": "label",
			"text": element.Text,
		}
	case form.Slider:
		return map[string]interface{}{
			"type":    "slider",
			"text":    element.Text,
			"min":     element.Min,
			"max":     element.Max,
			"step":    element.StepSize,
			"default": element.Default,
		}
	case form.Dropdown:
		return map[string]interface{}{
			"type":    "dropdown",
			"text":    element.Text,
			"default": element.DefaultIndex,
			"options": element.Options,
		}
	case form.StepSlider:
		return map[string]interface{}{
			"type":    "step_slider",
			"text":    element.Text,
			"default": element.DefaultIndex,
			"steps":   element.Options,
		}
	}
	panic("should never happen")
}

// Transfer transfers the player to a server with the IP and port passed.
func (s *Session) Transfer(ip net.IP, port int) {
	s.writePacket(&packet.Transfer{
		Address: ip.String(),
		Port:    uint16(port),
	})
}

// SendGameMode sends the game mode of the Controllable of the session to the client. It makes sure the right
// flags are set to create the full game mode.
func (s *Session) SendGameMode(mode gamemode.GameMode) {
	flags, id := uint32(0), int32(packet.GameTypeSurvival)
	switch mode.(type) {
	case gamemode.Creative:
		flags = packet.AdventureFlagAllowFlight
		id = packet.GameTypeCreative
	case gamemode.Adventure:
		flags |= packet.AdventureFlagWorldImmutable
		id = packet.GameTypeAdventure
	case gamemode.Spectator:
		flags, id = packet.AdventureFlagWorldImmutable|packet.AdventureFlagAllowFlight|packet.AdventureFlagMuted|packet.AdventureFlagNoClip|packet.AdventureFlagNoPVP, packet.GameTypeCreativeSpectator
	}
	s.adventureFlags.Store(flags)
	s.writePacket(&packet.AdventureSettings{
		Flags:             flags,
		PermissionLevel:   packet.PermissionLevelMember,
		PlayerUniqueID:    1,
		ActionPermissions: uint32(packet.ActionPermissionBuildAndMine | packet.ActionPermissionDoorsAndSwitched | packet.ActionPermissionOpenContainers | packet.ActionPermissionAttackPlayers | packet.ActionPermissionAttackMobs),
	})
	s.writePacket(&packet.SetPlayerGameType{GameType: id})
}

// SendHealth sends the health and max health to the player.
func (s *Session) SendHealth(health *entity_internal.HealthManager) {
	s.writePacket(&packet.UpdateAttributes{
		EntityRuntimeID: selfEntityRuntimeID,
		Attributes: []protocol.Attribute{{
			Name:    "minecraft:health",
			Value:   float32(math.Ceil(health.Health())),
			Max:     float32(math.Ceil(health.MaxHealth())),
			Default: 20,
		}},
	})
}

// SendAbsorption sends the absorption value passed to the player.
func (s *Session) SendAbsorption(value float64) {
	max := value
	if math.Mod(value, 2) != 0 {
		max = value + 1
	}
	s.writePacket(&packet.UpdateAttributes{
		EntityRuntimeID: selfEntityRuntimeID,
		Attributes: []protocol.Attribute{{
			Name:  "minecraft:absorption",
			Value: float32(math.Ceil(value)),
			Max:   float32(math.Ceil(max)),
		}},
	})
}

// SendEffect sends an effects passed to the player.
func (s *Session) SendEffect(e effect.Effect) {
	s.SendEffectRemoval(e)
	id, _ := effect_idByEffect(e)
	s.writePacket(&packet.MobEffect{
		EntityRuntimeID: selfEntityRuntimeID,
		Operation:       packet.MobEffectAdd,
		EffectType:      int32(id),
		Amplifier:       int32(e.Level() - 1),
		Particles:       e.ShowParticles(),
		Duration:        int32(e.Duration() / (time.Second / 20)),
	})
}

// SendEffectRemoval sends the removal of an effect passed.
func (s *Session) SendEffectRemoval(e effect.Effect) {
	id, ok := effect_idByEffect(e)
	if !ok {
		panic(fmt.Sprintf("unregistered effect type %T", e))
	}
	s.writePacket(&packet.MobEffect{
		EntityRuntimeID: selfEntityRuntimeID,
		Operation:       packet.MobEffectRemove,
		EffectType:      int32(id),
	})
}

// SendGameRules sends all the provided game rules to the player. Once sent, they will be immediately updated
// on the client if they are valid.
func (s *Session) sendGameRules(gameRules map[string]interface{}) {
	s.writePacket(&packet.GameRulesChanged{GameRules: gameRules})
}

// EnableCoordinates will either enable or disable coordinates for the player depending on the value given.
func (s *Session) EnableCoordinates(enable bool) {
	//noinspection SpellCheckingInspection
	s.sendGameRules(map[string]interface{}{"showcoordinates": enable})
}

// CanFly returns whether the client can fly.
func (s *Session) CanFly() bool {
	return packet.AdventureFlagAllowFlight&s.adventureFlags.Load() != 0
}

// SetCanFly sets whether or not the client can fly.
func (s *Session) SetCanFly(canfly bool) {
	if canfly {
		if !s.CanFly() { // Don't want to re-add it.
			s.adventureFlags.Store(s.adventureFlags.Load() | packet.AdventureFlagAllowFlight)
			s.updateAdventureSettings()
		}
	} else {
		if s.CanFly() { // Can't remove something that isn't there.
			s.adventureFlags.Store(s.adventureFlags.Load() &^ packet.AdventureFlagAllowFlight)
			s.updateAdventureSettings()
			s.SetFlying(false) // Stop them if they are already flying.
		}
	}
}

// Flying returns whether the client is currently flying or not.
func (s *Session) Flying() bool {
	return packet.AdventureFlagFlying&s.adventureFlags.Load() != 0
}

// SetFlying sets whether or not the client is allowed to fly.
func (s *Session) SetFlying(flying bool) {
	if flying {
		if !s.Flying() { // Don't want to re-add it.
			s.adventureFlags.Store(s.adventureFlags.Load() | packet.AdventureFlagFlying)
			s.updateAdventureSettings()
		}
	} else {
		if s.Flying() { // Can't remove something that isn't there.
			s.adventureFlags.Store(s.adventureFlags.Load() &^ packet.AdventureFlagFlying)
			s.updateAdventureSettings()
		}
	}
}

// updateAdventureSettings updates the adventure settings.
func (s *Session) updateAdventureSettings() {
	s.writePacket(&packet.AdventureSettings{
		Flags:             s.adventureFlags.Load(),
		PermissionLevel:   packet.PermissionLevelMember,
		PlayerUniqueID:    1,
		ActionPermissions: uint32(packet.ActionPermissionBuildAndMine | packet.ActionPermissionDoorsAndSwitched | packet.ActionPermissionOpenContainers | packet.ActionPermissionAttackPlayers | packet.ActionPermissionAttackMobs),
	})
}

// addToPlayerList adds the player of a session to the player list of this session. It will be shown in the
// in-game pause menu screen.
func (s *Session) addToPlayerList(session *Session) {
	c := session.c

	s.entityMutex.Lock()
	runtimeID := uint64(1)
	if session != s {
		runtimeID = s.currentEntityRuntimeID.Add(1)
	}
	s.entityRuntimeIDs[c] = runtimeID
	s.entities[runtimeID] = c
	s.entityMutex.Unlock()

	s.writePacket(&packet.PlayerList{
		ActionType: packet.PlayerListActionAdd,
		Entries: []protocol.PlayerListEntry{{
			UUID:           c.UUID(),
			EntityUniqueID: int64(runtimeID),
			Username:       c.Name(),
			XUID:           c.XUID(),
			Skin:           skinToProtocol(c.Skin()),
		}},
	})
}

// skinToProtocol converts a skin to its protocol representation.
func skinToProtocol(s skin.Skin) protocol.Skin {
	var animations []protocol.SkinAnimation
	for _, animation := range s.Animations {
		protocolAnim := protocol.SkinAnimation{
			ImageWidth:    uint32(animation.Bounds().Max.X),
			ImageHeight:   uint32(animation.Bounds().Max.Y),
			ImageData:     animation.Pix,
			AnimationType: 0,
			FrameCount:    float32(animation.FrameCount),
		}
		switch animation.Type() {
		case skin.AnimationHead:
			protocolAnim.AnimationType = protocol.SkinAnimationHead
		case skin.AnimationBody32x32:
			protocolAnim.AnimationType = protocol.SkinAnimationBody32x32
		case skin.AnimationBody128x128:
			protocolAnim.AnimationType = protocol.SkinAnimationBody128x128
		}
		animations = append(animations, protocolAnim)
	}

	return protocol.Skin{
		SkinID:            uuid.New().String(),
		SkinResourcePatch: s.ModelConfig.Encode(),
		SkinImageWidth:    uint32(s.Bounds().Max.X),
		SkinImageHeight:   uint32(s.Bounds().Max.Y),
		SkinData:          s.Pix,
		CapeImageWidth:    uint32(s.Cape.Bounds().Max.X),
		CapeImageHeight:   uint32(s.Cape.Bounds().Max.Y),
		CapeData:          s.Cape.Pix,
		SkinGeometry:      s.Model,
		PersonaSkin:       s.Persona,
		CapeID:            uuid.New().String(),
		FullSkinID:        uuid.New().String(),
		Animations:        animations,
		Trusted:           true,
	}
}

// protocolToSkin converts a protocol.Skin to a skin.Skin.
func protocolToSkin(data protocol.Skin) skin.Skin {
	modelConfig, _ := skin.DecodeModelConfig(data.SkinResourcePatch)

	playerSkin := skin.New(int(data.SkinImageWidth), int(data.SkinImageHeight))
	playerSkin.Persona = data.PersonaSkin
	playerSkin.Pix = data.SkinData
	playerSkin.Model = data.SkinGeometry
	playerSkin.ModelConfig = modelConfig

	for _, animation := range data.Animations {
		anim := skin.NewAnimation(int(animation.ImageWidth), int(animation.ImageHeight), skin.AnimationType(animation.AnimationType))
		anim.Pix = animation.ImageData
		anim.FrameCount = int(animation.FrameCount)
		playerSkin.Animations = append(playerSkin.Animations, anim)
	}
	return playerSkin
}

// removeFromPlayerList removes the player of a session from the player list of this session. It will no
// longer be shown in the in-game pause menu screen.
func (s *Session) removeFromPlayerList(session *Session) {
	c := session.c

	s.entityMutex.Lock()
	delete(s.entityRuntimeIDs, c)
	delete(s.entities, s.entityRuntimeIDs[c])
	s.entityMutex.Unlock()

	s.writePacket(&packet.PlayerList{
		ActionType: packet.PlayerListActionRemove,
		Entries: []protocol.PlayerListEntry{{
			UUID: c.UUID(),
		}},
	})
}

// HandleInventories starts handling the inventories of the Controllable of the session. It sends packets when
// slots in the inventory are changed.
func (s *Session) HandleInventories() (inv, offHand *inventory.Inventory, armour *inventory.Armour, heldSlot *atomic.Uint32) {
	s.inv = inventory.New(36, func(slot int, item item.Stack) {
		if slot == int(s.heldSlot.Load()) {
			for _, viewer := range s.c.World().Viewers(s.c.Position()) {
				viewer.ViewEntityItems(s.c)
			}
		}
		if !s.inTransaction.Load() {
			s.writePacket(&packet.InventorySlot{
				WindowID: protocol.WindowIDInventory,
				Slot:     uint32(slot),
				NewItem:  instanceFromItem(item),
			})
		}
	})
	s.offHand = inventory.New(2, func(slot int, item item.Stack) {
		for _, viewer := range s.c.World().Viewers(s.c.Position()) {
			viewer.ViewEntityItems(s.c)
		}
		if !s.inTransaction.Load() {
			i, _ := s.offHand.Item(1)
			s.writePacket(&packet.InventoryContent{
				WindowID: protocol.WindowIDOffHand,
				Content: []protocol.ItemInstance{
					instanceFromItem(i),
				},
			})
		}
	})
	s.armour = inventory.NewArmour(func(slot int, item item.Stack) {
		for _, viewer := range s.c.World().Viewers(s.c.Position()) {
			viewer.ViewEntityArmour(s.c)
		}
		if !s.inTransaction.Load() {
			s.writePacket(&packet.InventorySlot{
				WindowID: protocol.WindowIDArmour,
				Slot:     uint32(slot),
				NewItem:  instanceFromItem(item),
			})
		}
	})
	return s.inv, s.offHand, s.armour, s.heldSlot
}

// SetHeldSlot sets the currently held hotbar slot.
func (s *Session) SetHeldSlot(slot int) error {
	if slot > 8 {
		return fmt.Errorf("slot exceeds hotbar range 0-8: slot is %v", slot)
	}

	s.heldSlot.Store(uint32(slot))

	for _, viewer := range s.c.World().Viewers(s.c.Position()) {
		viewer.ViewEntityItems(s.c)
	}

	mainHand, _ := s.c.HeldItems()
	s.writePacket(&packet.MobEquipment{
		EntityRuntimeID: selfEntityRuntimeID,
		NewItem:         stackFromItem(mainHand),
		InventorySlot:   byte(slot),
		HotBarSlot:      byte(slot),
	})
	return nil
}

// stackFromItem converts an item.Stack to its network ItemStack representation.
func stackFromItem(it item.Stack) protocol.ItemStack {
	if it.Empty() {
		return protocol.ItemStack{}
	}
	id, meta := it.Item().EncodeItem()
	return protocol.ItemStack{
		ItemType: protocol.ItemType{
			NetworkID:     id,
			MetadataValue: meta,
		},
		Count:   int16(it.Count()),
		NBTData: nbtconv.ItemToNBT(it, true),
	}
}

// instanceFromItem converts an item.Stack to its network ItemInstance representation.
func instanceFromItem(it item.Stack) protocol.ItemInstance {
	return protocol.ItemInstance{
		StackNetworkID: item_id(it),
		Stack:          stackFromItem(it),
	}
}

// stackToItem converts a network ItemStack representation back to an item.Stack.
func stackToItem(it protocol.ItemStack) item.Stack {
	t, ok := world_itemByID(it.NetworkID, it.MetadataValue)
	if !ok {
		t = block.Air{}
	}
	//noinspection SpellCheckingInspection
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return nbtconv.ItemFromNBT(it.NBTData, &s)
}

// creativeItems returns all creative inventory items as protocol item stacks.
func creativeItems() []protocol.CreativeItem {
	it := make([]protocol.CreativeItem, 0, len(item.CreativeItems()))
	for index, i := range item.CreativeItems() {
		v := stackFromItem(i)
		delete(v.NBTData, "Damage")
		it = append(it, protocol.CreativeItem{
			CreativeItemNetworkID: uint32(index) + 1,
			Item:                  v,
		})
	}
	return it
}

// The following functions use the go:linkname directive in order to make sure the item.byID and item.toID
// functions do not need to be exported.

//go:linkname world_itemByID github.com/df-mc/dragonfly/dragonfly/world.itemByID
//noinspection ALL
func world_itemByID(id int32, meta int16) (world.Item, bool)

//go:linkname item_id github.com/df-mc/dragonfly/dragonfly/item.id
//noinspection ALL
func item_id(s item.Stack) int32

//go:linkname effect_idByEffect github.com/df-mc/dragonfly/dragonfly/entity/effect.idByEffect
//noinspection ALL
func effect_idByEffect(effect.Effect) (int, bool)

//go:linkname effect_byID github.com/df-mc/dragonfly/dragonfly/entity/effect.effectByID
//noinspection ALL
func effect_byID(int) (effect.Effect, bool)
