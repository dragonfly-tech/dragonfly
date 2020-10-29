package player

import (
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly/block"
	blockAction "github.com/df-mc/dragonfly/dragonfly/block/action"
	"github.com/df-mc/dragonfly/dragonfly/cmd"
	"github.com/df-mc/dragonfly/dragonfly/entity"
	"github.com/df-mc/dragonfly/dragonfly/entity/action"
	"github.com/df-mc/dragonfly/dragonfly/entity/damage"
	"github.com/df-mc/dragonfly/dragonfly/entity/effect"
	"github.com/df-mc/dragonfly/dragonfly/entity/healing"
	"github.com/df-mc/dragonfly/dragonfly/entity/physics"
	"github.com/df-mc/dragonfly/dragonfly/entity/state"
	"github.com/df-mc/dragonfly/dragonfly/event"
	"github.com/df-mc/dragonfly/dragonfly/internal/entity_internal"
	"github.com/df-mc/dragonfly/dragonfly/internal/nbtconv"
	"github.com/df-mc/dragonfly/dragonfly/item"
	"github.com/df-mc/dragonfly/dragonfly/item/armour"
	"github.com/df-mc/dragonfly/dragonfly/item/inventory"
	"github.com/df-mc/dragonfly/dragonfly/item/tool"
	"github.com/df-mc/dragonfly/dragonfly/player/bossbar"
	"github.com/df-mc/dragonfly/dragonfly/player/chat"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/df-mc/dragonfly/dragonfly/player/scoreboard"
	"github.com/df-mc/dragonfly/dragonfly/player/skin"
	"github.com/df-mc/dragonfly/dragonfly/player/title"
	"github.com/df-mc/dragonfly/dragonfly/session"
	"github.com/df-mc/dragonfly/dragonfly/world"
	"github.com/df-mc/dragonfly/dragonfly/world/difficulty"
	"github.com/df-mc/dragonfly/dragonfly/world/gamemode"
	"github.com/df-mc/dragonfly/dragonfly/world/particle"
	"github.com/df-mc/dragonfly/dragonfly/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"go.uber.org/atomic"
	"image/color"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// Player is an implementation of a player entity. It has methods that implement the behaviour that players
// need to play in the world.
type Player struct {
	name                         string
	uuid                         uuid.UUID
	xuid                         string
	pos, velocity                atomic.Value
	nameTag                      atomic.String
	yaw, pitch, absorptionHealth atomic.Float64

	gameModeMu sync.RWMutex
	gameMode   gamemode.GameMode

	skin skin.Skin

	sMutex sync.RWMutex
	// s holds the session of the player. This field should not be used directly, but instead,
	// Player.session() should be called.
	s *session.Session

	hMutex sync.RWMutex
	// h holds the current handler of the player. It may be changed at any time by calling the Start method.
	h Handler

	inv, offHand *inventory.Inventory
	armour       *inventory.Armour
	heldSlot     *atomic.Uint32

	sneaking, sprinting, swimming, invisible,
	immobile, onGround, usingItem atomic.Bool
	usingSince atomic.Int64

	fireTicks atomic.Int64

	speed    atomic.Float64
	scale    atomic.Float64
	health   *entity_internal.HealthManager
	effects  *entity.EffectManager
	immunity atomic.Value

	breaking          atomic.Bool
	breakingPos       atomic.Value
	lastBreakDuration time.Duration

	breakParticleCounter atomic.Uint32

	hunger *hungerManager
}

// New returns a new initialised player. A random UUID is generated for the player, so that it may be
// identified over network.
func New(name string, skin skin.Skin, pos mgl64.Vec3) *Player {
	p := &Player{}
	*p = Player{
		inv: inventory.New(36, func(slot int, item item.Stack) {
			if slot == int(p.heldSlot.Load()) {
				p.broadcastItems(slot, item)
			}
		}),
		uuid:     uuid.New(),
		offHand:  inventory.New(2, p.broadcastItems),
		armour:   inventory.NewArmour(p.broadcastArmour),
		hunger:   newHungerManager(),
		health:   entity_internal.NewHealthManager(),
		effects:  entity.NewEffectManager(),
		gameMode: gamemode.Adventure{},
		h:        NopHandler{},
		name:     name,
		skin:     skin,
		speed:    *atomic.NewFloat64(0.1),
		nameTag:  *atomic.NewString(name),
		heldSlot: atomic.NewUint32(0),
		scale:    *atomic.NewFloat64(1),
	}
	p.pos.Store(pos)
	p.velocity.Store(mgl64.Vec3{})
	p.immunity.Store(time.Now())
	p.breakingPos.Store(world.BlockPos{})
	return p
}

// NewWithSession returns a new player for a network session, so that the network session can control the
// player.
// A set of additional fields must be provided to initialise the player with the client's data, such as the
// name and the skin of the player.
func NewWithSession(name, xuid string, uuid uuid.UUID, skin skin.Skin, s *session.Session, pos mgl64.Vec3) *Player {
	p := New(name, skin, pos)
	p.s, p.uuid, p.xuid, p.skin = s, uuid, xuid, skin
	p.inv, p.offHand, p.armour, p.heldSlot = s.HandleInventories()

	chat.Global.Subscribe(p)
	return p
}

// Name returns the username of the player. If the player is controlled by a client, it is the username of
// the client. (Typically the XBOX Live name)
func (p *Player) Name() string {
	return p.name
}

// UUID returns the UUID of the player. This UUID will remain consistent with an XBOX Live account, and will,
// unlike the name of the player, never change.
// It is therefore recommended to use the UUID over the name of the player. Additionally, it is recommended to
// use the UUID over the XUID because of its standard format.
func (p *Player) UUID() uuid.UUID {
	return p.uuid
}

// XUID returns the XBOX Live user ID of the player. It will remain consistent with the XBOX Live account,
// and will not change in the lifetime of an account.
// The XUID is a number that can be parsed as an int64. No more information on what it represents is
// available, and the UUID should be preferred.
// The XUID returned is empty if the Player is not connected to a network session or if the Player is not
// authenticated with XBOX Live.
func (p *Player) XUID() string {
	return p.xuid
}

// Skin returns the current skin for a player.
func (p *Player) Skin() skin.Skin {
	return p.skin
}

// SetSkin updates a players skin.
func (p *Player) SetSkin(new skin.Skin) {
	p.skin = new
	for _, viewer := range p.World().Viewers(p.Position()) {
		viewer.ViewEntitySkin(p)
	}
}

// Handle changes the current handler of the player. As a result, events called by the player will call
// handlers of the Handler passed.
// Handle sets the player's handler to NopHandler if nil is passed.
func (p *Player) Handle(h Handler) {
	p.hMutex.Lock()
	defer p.hMutex.Unlock()

	if h == nil {
		h = NopHandler{}
	}
	p.h = h
}

// Message sends a formatted message to the player. The message is formatted following the rules of
// fmt.Sprintln, however the newline at the end is not written.
func (p *Player) Message(a ...interface{}) {
	p.session().SendMessage(format(a))
}

// Messagef sends a formatted message using a specific format to the player. The message is formatted
// according to the fmt.Sprintf formatting rules.
func (p *Player) Messagef(f string, a ...interface{}) {
	msg := fmt.Sprintf(f, a...)
	p.session().SendMessage(msg)
}

// SendPopup sends a formatted popup to the player. The popup is shown above the hotbar of the player and
// overwrites/is overwritten by the name of the item equipped.
// The popup is formatted following the rules of fmt.Sprintln without a newline at the end.
func (p *Player) SendPopup(a ...interface{}) {
	p.session().SendPopup(format(a))
}

// SendTip sends a tip to the player. The tip is shown in the middle of the screen of the player.
// The tip is formatted following the rules of fmt.Sprintln without a newline at the end.
func (p *Player) SendTip(a ...interface{}) {
	p.session().SendTip(format(a))
}

// SendTitle sends a title to the player. The title may be configured to change the duration it is displayed
// and the text it shows.
// If non-empty, the subtitle is shown in a smaller font below the title. The same counts for the action text
// of the title, which is shown in a font similar to that of a tip/popup.
func (p *Player) SendTitle(t title.Title) {
	p.session().SetTitleDurations(t.FadeInDuration(), t.Duration(), t.FadeOutDuration())
	p.session().SendTitle(t.Text())
	if t.Subtitle() != "" {
		p.session().SendSubtitle(t.Subtitle())
	}
	if t.ActionText() != "" {
		p.session().SendActionBarMessage(t.ActionText())
	}
}

// SendScoreboard sends a scoreboard to the player. The scoreboard will be present indefinitely until removed
// by the caller.
// SendScoreboard may be called at any time to change the scoreboard of the player.
func (p *Player) SendScoreboard(scoreboard *scoreboard.Scoreboard) {
	p.session().SendScoreboard(scoreboard.Name())
	p.session().SendScoreboardLines(scoreboard.Lines())
}

// RemoveScoreboard removes any scoreboard currently present on the screen of the player. Nothing happens if
// the player has no scoreboard currently active.
func (p *Player) RemoveScoreboard() {
	p.session().RemoveScoreboard()
}

// SendBossBar sends a boss bar to the player, so that it will be shown indefinitely at the top of the
// player's screen.
// The boss bar may be removed by calling Player.RemoveBossBar().
func (p *Player) SendBossBar(bar bossbar.BossBar) {
	p.session().SendBossBar(bar.Text(), bar.HealthPercentage())
}

// RemoveBossBar removes any boss bar currently active on the player's screen. If no boss bar is currently
// present, nothing happens.
func (p *Player) RemoveBossBar() {
	p.session().RemoveBossBar()
}

// Chat writes a message in the global chat (chat.Global). The message is prefixed with the name of the
// player and is formatted following the rules of fmt.Sprintln.
func (p *Player) Chat(msg ...interface{}) {
	if p.Dead() {
		return
	}
	message := format(msg)
	ctx := event.C()
	p.handler().HandleChat(ctx, &message)

	ctx.Continue(func() {
		chat.Global.Printf("<%v> %v\n", p.name, message)
	})
}

// ExecuteCommand executes a command passed as the player. If the command could not be found, or if the usage
// was incorrect, an error message is sent to the player.
func (p *Player) ExecuteCommand(commandLine string) {
	if p.Dead() {
		return
	}
	args := strings.Split(commandLine, " ")
	commandName := strings.TrimPrefix(args[0], "/")

	command, ok := cmd.ByAlias(commandName)
	if !ok {
		output := &cmd.Output{}
		output.Errorf("Unknown command '%v'", commandName)
		p.SendCommandOutput(output)
		return
	}

	ctx := event.C()
	p.handler().HandleCommandExecution(ctx, command, args[1:])
	ctx.Continue(func() {
		command.Execute(strings.TrimPrefix(strings.TrimPrefix(commandLine, "/"+commandName), " "), p)
	})
}

// Disconnect closes the player and removes it from the world.
// Disconnect, unlike Close, allows a custom message to be passed to show to the player when it is
// disconnected. The message is formatted following the rules of fmt.Sprintln without a newline at the end.
func (p *Player) Disconnect(msg ...interface{}) {
	p.session().Disconnect(format(msg))
	p.close()
}

// Transfer transfers the player to a server at the address passed. If the address could not be resolved, an
// error is returned. If it is returned, the player is closed and transferred to the server.
func (p *Player) Transfer(address string) (err error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	ctx := event.C()
	p.handler().HandleTransfer(ctx, addr)

	ctx.Continue(func() {
		p.session().Transfer(addr.IP, addr.Port)
		err = p.Close()
	})
	return
}

// SendCommandOutput sends the output of a command to the player.
func (p *Player) SendCommandOutput(output *cmd.Output) {
	p.session().SendCommandOutput(output)
}

// SendForm sends a form to the player for the client to fill out. Once the client fills it out, the Submit
// method of the form will be called.
// Note that the client may also close the form instead of filling it out, which will result in the form not
// having its Submit method called at all. Forms should never depend on the player actually filling out the
// form.
func (p *Player) SendForm(f form.Form) {
	p.session().SendForm(f)
}

// ShowCoordinates enables the vanilla coordinates for the player.
func (p *Player) ShowCoordinates() {
	p.session().EnableCoordinates(true)
}

// HideCoordinates disables the vanilla coordinates for the player.
func (p *Player) HideCoordinates() {
	p.session().EnableCoordinates(false)
}

// SetNameTag changes the name tag displayed over the player in-game. Changing the name tag does not change
// the player's name in, for example, the player list or the chat.
func (p *Player) SetNameTag(name string) {
	p.nameTag.Store(name)
	p.updateState()
}

// SetSpeed sets the speed of the player. The value passed is the blocks/tick speed that the player will then
// obtain.
func (p *Player) SetSpeed(speed float64) {
	p.speed.Store(speed)
	p.session().SendSpeed(speed)
}

// Speed returns the speed of the player, returning a value that indicates the blocks/tick speed. The default
// speed of a player is 0.1.
func (p *Player) Speed() float64 {
	return p.speed.Load()
}

// SetScale sets the scale of the player's geometry.
func (p *Player) SetScale(scale float64) {
	p.scale.Store(scale)
	p.updateState()
}

// Scale returns the current scale of the player.
func (p *Player) Scale() float64 {
	return p.scale.Load()
}

// Health returns the current health of the player. It will always be lower than Player.MaxHealth().
func (p *Player) Health() float64 {
	return p.health.Health()
}

// MaxHealth returns the maximum amount of health that a player may have. The MaxHealth will always be higher
// than Player.Health().
func (p *Player) MaxHealth() float64 {
	return p.health.MaxHealth()
}

// SetMaxHealth sets the maximum health of the player. If the current health of the player is higher than the
// new maximum health, the health is set to the new maximum.
// SetMaxHealth panics if the max health passed is 0 or lower.
func (p *Player) SetMaxHealth(health float64) {
	p.health.SetMaxHealth(health)
	p.session().SendHealth(p.health)
}

// addHealth adds health to the player's current health.
func (p *Player) addHealth(health float64) {
	p.health.AddHealth(health)
	p.session().SendHealth(p.health)
}

// Heal heals the entity for a given amount of health. The source passed represents the cause of the
// healing, for example healing.SourceFood if the entity healed by having a full food bar. If the health
// added to the original health exceeds the entity's max health, Heal will not add the full amount.
// If the health passed is negative, Heal will not do anything.
func (p *Player) Heal(health float64, source healing.Source) {
	if p.Dead() || health < 0 || !p.survival() {
		return
	}
	ctx := event.C()
	p.handler().HandleHeal(ctx, &health, source)
	ctx.Continue(func() {
		p.addHealth(health)
	})
}

// Hurt hurts the player for a given amount of damage. The source passed represents the cause of the damage,
// for example damage.SourceEntityAttack if the player is attacked by another entity.
// If the final damage exceeds the health that the player currently has, the player is killed and will have to
// respawn.
// If the damage passed is negative, Hurt will not do anything.
func (p *Player) Hurt(dmg float64, source damage.Source) {
	if p.Dead() || dmg < 0 || !p.survival() {
		return
	}
	for _, e := range p.Effects() {
		if _, ok := e.(effect.FireResistance); ok && (source == damage.SourceFire{} || source == damage.SourceFireTick{} || source == damage.SourceLava{}) {
			return
		}
	}

	ctx := event.C()
	p.handler().HandleHurt(ctx, &dmg, source)

	ctx.Continue(func() {
		if source.ReducedByArmour() {
			p.Exhaust(0.1)
		}
		finalDamage := p.FinalDamageFrom(dmg, source)

		a := p.absorption()
		if a > 0 && (effect.Absorption{}).Absorbs(source) {
			if finalDamage > a {
				finalDamage -= a
				p.SetAbsorption(0)
				p.effects.Remove(effect.Absorption{}, p)
			} else {
				p.SetAbsorption(a - finalDamage)
				finalDamage = 0
			}
		}
		p.addHealth(-finalDamage)

		for _, viewer := range p.World().Viewers(p.Position()) {
			viewer.ViewEntityAction(p, action.Hurt{})
		}
		p.immunity.Store(time.Now().Add(time.Second / 2))
		if p.Dead() {
			p.kill(source)
		}
	})
}

// FinalDamageFrom resolves the final damage received by the player if it is attacked by the source passed
// with the damage passed. FinalDamageFrom takes into account things such as the armour worn and the
// enchantments on the individual pieces.
// The damage returned will be at the least 0.
func (p *Player) FinalDamageFrom(dmg float64, src damage.Source) float64 {
	if src.ReducedByArmour() {
		defencePoints, damageToArmour := 0.0, int(dmg/4)
		if damageToArmour == 0 {
			damageToArmour++
		}
		for i := 0; i < 4; i++ {
			it, _ := p.armour.Inv().Item(i)
			if a, ok := it.Item().(armour.Armour); ok {
				defencePoints += a.DefencePoints()
				if _, ok := it.Item().(item.Durable); ok {
					_ = p.armour.Inv().SetItem(i, p.damageItem(it, damageToArmour))
				}
			}
		}
		// Armour in Bedrock edition reduces the damage taken by 4% for every armour point that the player
		// has, with a maximum of 4*20=80%
		dmg -= dmg * 0.04 * defencePoints
	}
	for _, e := range p.Effects() {
		if resistance, ok := e.(effect.Resistance); ok {
			dmg *= resistance.Multiplier(src)
		}
	}
	// TODO: Account for enchantments.
	if dmg < 0 {
		dmg = 0
	}
	return dmg
}

// SetAbsorption sets the absorption health of a player. This extra health shows as golden hearts and do not
// actually increase the maximum health. Once the hearts are lost, they will not regenerate.
// Nothing happens if a negative number is passed.
func (p *Player) SetAbsorption(health float64) {
	if health < 0 {
		return
	}
	p.absorptionHealth.Store(health)
	p.session().SendAbsorption(health)
}

// absorption returns the absorption health that the player has.
func (p *Player) absorption() float64 {
	return p.absorptionHealth.Load()
}

// KnockBack knocks the player back with a given force and height. A source is passed which indicates the
// source of the velocity, typically the position of an attacking entity. The source is used to calculate the
// direction which the entity should be knocked back in.
func (p *Player) KnockBack(src mgl64.Vec3, force, height float64) {
	if p.Dead() || !p.survival() {
		return
	}
	if p.session() == session.Nop {
		// TODO: Implement server-side movement and knock-back.
		return
	}
	velocity := p.Position().Sub(src)
	velocity[1] = 0
	velocity = velocity.Normalize().Mul(force)
	velocity[1] = height

	resistance := 0.0
	for _, i := range p.armour.All() {
		if a, ok := i.Item().(armour.Armour); ok {
			resistance += a.KnockBackResistance()
		}
	}
	p.session().SendVelocity(velocity.Mul(1 - resistance))
}

// AttackImmune checks if the player is currently immune to entity attacks, meaning it was recently attacked.
func (p *Player) AttackImmune() bool {
	return p.immunity.Load().(time.Time).After(time.Now())
}

// Food returns the current food level of a player. The level returned is guaranteed to always be between 0
// and 20. Every half drumstick is one level.
func (p *Player) Food() int {
	return p.hunger.Food()
}

// SetFood sets the food level of a player. The level passed must be in a range of 0-20. If the level passed
// is negative, the food level will be set to 0. If the level exceeds 20, the food level will be set to 20.
func (p *Player) SetFood(level int) {
	p.hunger.SetFood(level)
	p.sendFood()
}

// AddFood adds a number of points to the food level of the player. If the new food level is negative or if
// it exceeds 20, it will be set to 0 or 20 respectively.
func (p *Player) AddFood(points int) {
	p.hunger.AddFood(points)
	p.sendFood()
}

// Saturate saturates the player's food bar with the amount of food points and saturation points passed. The
// total saturation of the player will never exceed its total food level.
func (p *Player) Saturate(food int, saturation float64) {
	p.hunger.saturate(food, saturation)
	p.sendFood()
}

// sendFood sends the current food properties to the client.
func (p *Player) sendFood() {
	p.hunger.mu.RLock()
	defer p.hunger.mu.RUnlock()
	p.session().SendFood(p.hunger.foodLevel, p.hunger.saturationLevel, p.hunger.exhaustionLevel)
}

// AddEffect adds an entity.Effect to the Player. If the effect is instant, it is applied to the Player
// immediately. If not, the effect is applied to the player every time the Tick method is called.
// AddEffect will overwrite any effects present if the level of the effect is higher than the existing one, or
// if the effects' levels are equal and the new effect has a longer duration.
func (p *Player) AddEffect(e effect.Effect) {
	p.session().SendEffect(p.effects.Add(e, p))
	p.updateState()
}

// RemoveEffect removes any effect that might currently be active on the Player.
func (p *Player) RemoveEffect(e effect.Effect) {
	p.effects.Remove(e, p)
	p.session().SendEffectRemoval(e)
	p.updateState()
}

// Effects returns any effect currently applied to the entity. The returned effects are guaranteed not to have
// expired when returned.
func (p *Player) Effects() []effect.Effect {
	return p.effects.Effects()
}

// BeaconAffected ...
func (*Player) BeaconAffected() bool {
	return true
}

// Exhaust exhausts the player by the amount of points passed if the player is in survival mode. If the total
// exhaustion level exceeds 4, a saturation point, or food point, if saturation is 0, will be subtracted.
func (p *Player) Exhaust(points float64) {
	if !p.survival() {
		return
	}
	before := p.hunger.Food()
	if (p.World().Difficulty() != difficulty.Peaceful{}) {
		p.hunger.exhaust(points)
	}
	after := p.hunger.Food()
	if before != after {
		// Temporarily set the food level back so that it hasn't yet changed once the event is handled.
		p.hunger.SetFood(before)

		ctx := event.C()
		p.handler().HandleFoodLoss(ctx, before, after)
		ctx.Continue(func() {
			p.hunger.SetFood(after)
			if before >= 7 && after <= 6 {
				// The client will stop sprinting by itself too, but we force it just to be sure.
				p.StopSprinting()
			}
		})
	}
	p.sendFood()
}

// survival checks if the player is considered to be survival, meaning either adventure or survival game mode.
func (p *Player) survival() bool {
	return p.GameMode() == gamemode.Survival{} || p.GameMode() == gamemode.Adventure{}
}

// canEdit checks if the player has a game mode that allows it to edit the world.
func (p *Player) canEdit() bool {
	return p.GameMode() == gamemode.Creative{} || p.GameMode() == gamemode.Survival{}
}

// Dead checks if the player is considered dead. True is returned if the health of the player is equal to or
// lower than 0.
func (p *Player) Dead() bool {
	return p.Health() <= 0
}

// kill kills the player, clearing its inventories and resetting it to its base state.
func (p *Player) kill(src damage.Source) {
	for _, viewer := range p.World().Viewers(p.Position()) {
		viewer.ViewEntityAction(p, action.Death{})
	}

	p.addHealth(-p.MaxHealth())
	p.StopSneaking()
	p.StopSprinting()
	p.inv.Clear()
	p.armour.Clear()
	p.offHand.Clear()
	for _, e := range p.Effects() {
		p.RemoveEffect(e)
	}

	p.handler().HandleDeath(src)

	// Wait for a little bit before removing the entity. The client displays a death animation while the
	// player is dying.
	time.AfterFunc(time.Millisecond*1100, func() {
		if p.session() == session.Nop {
			_ = p.Close()
			return
		}
		if p.Dead() {
			p.SetInvisible()
			// We have an actual client connected to this player: We change its position server side so that in
			// the future, the client won't respawn on the death location when disconnecting. The client should
			// not see the movement itself yet, though.
			p.pos.Store(p.World().Spawn().Vec3())
		}
	})
}

// Respawn spawns the player after it dies, so that its health is replenished and it is spawned in the world
// again. Nothing will happen if the player does not have a session connected to it.
func (p *Player) Respawn() {
	if !p.Dead() || p.World() == nil || p.session() == session.Nop {
		return
	}
	pos := p.World().Spawn().Vec3Middle()
	p.handler().HandleRespawn(&pos)
	p.addHealth(p.MaxHealth())
	p.hunger.Reset()
	p.sendFood()
	p.Extinguish()

	p.World().AddEntity(p)
	p.SetVisible()

	p.Teleport(pos)
	p.session().SendRespawn()
}

// StartSprinting makes a player start sprinting, increasing the speed of the player by 30% and making
// particles show up under the feet. The player will only start sprinting if its food level is high enough.
// If the player is sneaking when calling StartSprinting, it is stopped from sneaking.
func (p *Player) StartSprinting() {
	if !p.hunger.canSprint() && (p.GameMode() != gamemode.Creative{}) {
		return
	}
	if !p.sprinting.CAS(false, true) {
		return
	}
	p.StopSneaking()
	p.SetSpeed(p.Speed() * 1.3)

	p.updateState()
}

// Sprinting checks if the player is currently sprinting.
func (p *Player) Sprinting() bool {
	return p.sprinting.Load()
}

// StopSprinting makes a player stop sprinting, setting back the speed of the player to its original value.
func (p *Player) StopSprinting() {
	if !p.sprinting.CAS(true, false) {
		return
	}
	p.SetSpeed(p.Speed() / 1.3)

	p.updateState()
}

// StartSneaking makes a player start sneaking. If the player is already sneaking, StartSneaking will not do
// anything.
// If the player is sprinting while StartSneaking is called, the sprinting is stopped.
func (p *Player) StartSneaking() {
	if !p.sneaking.CAS(false, true) {
		return
	}
	p.StopSprinting()
	p.updateState()
}

// Sneaking checks if the player is currently sneaking.
func (p *Player) Sneaking() bool {
	return p.sneaking.Load()
}

// StopSneaking makes a player stop sneaking if it currently is. If the player is not sneaking, StopSneaking
// will not do anything.
func (p *Player) StopSneaking() {
	if !p.sneaking.CAS(true, false) {
		return
	}
	p.updateState()
}

// StartSwimming makes the player start swimming if it is not currently doing so. If the player is sneaking
// while StartSwimming is called, the sneaking is stopped.
func (p *Player) StartSwimming() {
	if !p.swimming.CAS(false, true) {
		return
	}
	p.StopSneaking()
	p.updateState()
}

// Swimming checks if the player is currently swimming.
func (p *Player) Swimming() bool {
	return p.swimming.Load()
}

// StopSwimming makes the player stop swimming if it is currently doing so.
func (p *Player) StopSwimming() {
	if !p.swimming.CAS(true, false) {
		return
	}
	p.updateState()
}

// SetInvisible sets the player invisible, so that other players will not be able to see it.
func (p *Player) SetInvisible() {
	if !p.invisible.CAS(false, true) {
		return
	}
	p.updateState()
}

// SetVisible sets the player visible again, so that other players can see it again. If the player was already
// visible, or if the player is in spectator mode, nothing happens.
func (p *Player) SetVisible() {
	if (p.GameMode() == gamemode.Spectator{}) {
		return
	}
	if !p.invisible.CAS(true, false) {
		return
	}
	p.updateState()
}

// SetImmobile prevents the player from moving around, but still allows them to look around.
func (p *Player) SetImmobile() {
	if !p.immobile.CAS(false, true) {
		return
	}
	p.updateState()
}

// SetMobile allows the player to freely move around again after being immobile.
func (p *Player) SetMobile() {
	if !p.immobile.CAS(true, false) {
		return
	}
	p.updateState()
}

// FireProof ...
func (p *Player) FireProof() bool {
	return p.GameMode() != gamemode.Survival{} && p.GameMode() != gamemode.Adventure{}
}

// OnFireDuration ...
func (p *Player) OnFireDuration() time.Duration {
	return time.Duration(p.fireTicks.Load()) * time.Second / 20
}

// SetOnFire ...
func (p *Player) SetOnFire(duration time.Duration) {
	p.fireTicks.Store(int64(duration.Seconds() * 20))
	p.updateState()
}

// Extinguish ...
func (p *Player) Extinguish() {
	p.SetOnFire(0)
}

// Inventory returns the inventory of the player. This inventory holds the items stored in the normal part of
// the inventory and the hotbar. It also includes the item in the main hand as returned by Player.HeldItems().
func (p *Player) Inventory() *inventory.Inventory {
	return p.inv
}

// Armour returns the armour inventory of the player. This inventory yields 4 slots, for the helmet,
// chestplate, leggings and boots respectively.
func (p *Player) Armour() item.ArmourContainer {
	return p.armour
}

// HeldItems returns the items currently held in the hands of the player. The first item stack returned is the
// one held in the main hand, the second is held in the off-hand.
// If no item was held in a hand, the stack returned has a count of 0. Stack.Empty() may be used to check if
// the hand held anything.
func (p *Player) HeldItems() (mainHand, offHand item.Stack) {
	offHand, _ = p.offHand.Item(1)
	mainHand, _ = p.inv.Item(int(p.heldSlot.Load()))
	return mainHand, offHand
}

// SetHeldItems sets items to the main hand and the off-hand of the player. The Stacks passed may be empty
// (Stack.Empty()) to clear the held item.
func (p *Player) SetHeldItems(mainHand, offHand item.Stack) {
	_ = p.inv.SetItem(int(p.heldSlot.Load()), mainHand)
	_ = p.offHand.SetItem(1, offHand)
}

// SetGameMode sets the game mode of a player. The game mode specifies the way that the player can interact
// with the world that it is in.
func (p *Player) SetGameMode(mode gamemode.GameMode) {
	p.gameModeMu.Lock()
	previous := p.gameMode
	p.gameMode = mode
	p.gameModeMu.Unlock()

	p.session().SendGameMode(mode)

	if (mode == gamemode.Spectator{}) {
		p.SetInvisible()
	} else if (mode != gamemode.Spectator{}) && (previous == gamemode.Spectator{}) {
		for _, eff := range p.Effects() {
			if _, ok := eff.(effect.Invisibility); ok {
				return
			}
		}
		p.SetVisible()
	}
}

// GameMode returns the current game mode assigned to the player. If not changed, the game mode returned will
// be the same as that of the world that the player spawns in.
// The game mode may be changed using Player.SetGameMode().
func (p *Player) GameMode() gamemode.GameMode {
	p.gameModeMu.RLock()
	mode := p.gameMode
	p.gameModeMu.RUnlock()
	return mode
}

// UseItem uses the item currently held in the player's main hand in the air. Generally, nothing happens,
// unless the held item implements the item.Usable interface, in which case it will be activated.
// This generally happens for items such as throwable items like snowballs.
func (p *Player) UseItem() {
	if !p.canReach(p.Position()) {
		return
	}
	i, left := p.HeldItems()
	ctx := event.C()
	p.handler().HandleItemUse(ctx)

	ctx.Continue(func() {
		switch usable := i.Item().(type) {
		case item.Usable:
			ctx := &item.UseContext{}
			if usable.Use(p.World(), p, ctx) {
				// We only swing the player's arm if the item held actually does something. If it doesn't, there is no
				// reason to swing the arm.
				p.swingArm()

				p.SetHeldItems(p.subtractItem(p.damageItem(i, ctx.Damage), ctx.CountSub), left)
				p.addNewItem(ctx)
			}
		case item.Consumable:
			if !usable.AlwaysConsumable() && (p.GameMode() != gamemode.Creative{}) && p.Food() >= 20 {
				// The item.Consumable is not always consumable, the player is not in creative mode and the
				// food bar is filled: The item cannot be consumed.
				p.ReleaseItem()
				return
			}
			if !p.usingItem.CAS(false, true) {
				// The player is currently using the item held. This is a signal the item was consumed, so we
				// consume it and start using it again.
				// Due to the network overhead and latency, the duration might sometimes be a little off. We
				// slightly increase the duration to combat this.
				duration := time.Duration(time.Now().UnixNano()-p.usingSince.Load()) + time.Second/20

				held, left := p.HeldItems()
				if duration < usable.ConsumeDuration() {
					// The required duration for consuming this item was not met, so we don't consume it.
					return
				}
				p.SetHeldItems(p.subtractItem(held, 1), left)
				p.addNewItem(&item.UseContext{NewItem: usable.Consume(p.World(), p)})
				p.World().PlaySound(p.Position().Add(mgl64.Vec3{0, 1.5}), sound.Burp{})
				return
			}
			p.usingSince.Store(time.Now().UnixNano())
			p.updateState()
		}
	})
}

// ReleaseItem makes the Player release the item it is currently using. This is only applicable for items that
// implement the item.Consumable interface.
// If the Player is not currently using any item, ReleaseItem returns immediately.
// ReleaseItem either aborts the using of the item or finished it, depending on the time that elapsed since
// the item started being used.
func (p *Player) ReleaseItem() {
	if p.usingItem.CAS(true, false) {
		p.updateState()

		// TODO: Release items such as bows.
	}
}

// UseItemOnBlock uses the item held in the main hand of the player on a block at the position passed. The
// player is assumed to have clicked the face passed with the relative click position clickPos.
// If the item could not be used successfully, for example when the position is out of range, the method
// returns immediately.
func (p *Player) UseItemOnBlock(pos world.BlockPos, face world.Face, clickPos mgl64.Vec3) {
	if !p.canReach(pos.Vec3Centre()) {
		return
	}
	i, left := p.HeldItems()

	ctx := event.C()
	p.handler().HandleItemUseOnBlock(ctx, pos, face, clickPos)

	ctx.Continue(func() {
		if activatable, ok := p.World().Block(pos).(block.Activatable); ok {
			// If a player is sneaking, it will not activate the block clicked, unless it is not holding any
			// items, in which the block will activated as usual.
			if !p.Sneaking() || i.Empty() {
				p.swingArm()
				// The block was activated: Blocks such as doors must always have precedence over the item being
				// used.
				activatable.Activate(pos, face, p.World(), p)
				return
			}
		}
		if i.Empty() {
			return
		}
		if usableOnBlock, ok := i.Item().(item.UsableOnBlock); ok {
			// The item does something when used on a block.
			ctx := &item.UseContext{}
			if usableOnBlock.UseOnBlock(pos, face, clickPos, p.World(), p, ctx) {
				p.swingArm()
				p.SetHeldItems(p.subtractItem(p.damageItem(i, ctx.Damage), ctx.CountSub), left)
				p.addNewItem(ctx)
			}
		} else if b, ok := i.Item().(world.Block); ok && p.canEdit() {
			// The item IS a block, meaning it is being placed.
			replacedPos := pos
			if replaceable, ok := p.World().Block(pos).(block.Replaceable); !ok || !replaceable.ReplaceableBy(b) {
				// The block clicked was either not replaceable, or not replaceable using the block passed.
				replacedPos = pos.Side(face)
			}
			if replaceable, ok := p.World().Block(replacedPos).(block.Replaceable); ok && replaceable.ReplaceableBy(b) && !replacedPos.OutOfBounds() {
				if p.placeBlock(replacedPos, b, false) && p.survival() {
					p.SetHeldItems(p.subtractItem(i, 1), left)
				}
			}
		}
	})
	ctx.Stop(func() {
		p.World().SetBlock(pos, p.World().Block(pos))
		p.World().SetBlock(pos.Side(face), p.World().Block(pos.Side(face)))
		if liq, ok := p.World().Liquid(pos); ok {
			p.World().SetLiquid(pos, liq)
		}
		if liq, ok := p.World().Liquid(pos.Side(face)); ok {
			p.World().SetLiquid(pos.Side(face), liq)
		}
	})
}

// UseItemOnEntity uses the item held in the main hand of the player on the entity passed, provided it is
// within range of the player.
// If the item held in the main hand of the player does nothing when used on an entity, nothing will happen.
func (p *Player) UseItemOnEntity(e world.Entity) {
	if !p.canReach(e.Position()) {
		return
	}
	i, left := p.HeldItems()

	ctx := event.C()
	p.handler().HandleItemUseOnEntity(ctx, e)

	ctx.Continue(func() {
		if usableOnEntity, ok := i.Item().(item.UsableOnEntity); ok {
			ctx := &item.UseContext{}
			if usableOnEntity.UseOnEntity(e, e.World(), p, ctx) {
				p.swingArm()
				p.SetHeldItems(p.subtractItem(p.damageItem(i, ctx.Damage), ctx.CountSub), left)
				p.addNewItem(ctx)
			}
		}
	})
}

// AttackEntity uses the item held in the main hand of the player to attack the entity passed, provided it is
// within range of the player.
// The damage dealt to the entity will depend on the item held by the player and any effects the player may
// have.
// If the player cannot reach the entity at its position, the method returns immediately.
func (p *Player) AttackEntity(e world.Entity) {
	if !p.canReach(e.Position()) {
		return
	}
	i, left := p.HeldItems()

	ctx := event.C()
	p.handler().HandleAttackEntity(ctx, e)
	ctx.Continue(func() {
		p.swingArm()
		living, ok := e.(entity.Living)
		if !ok {
			return
		}
		if living.AttackImmune() {
			return
		}
		p.StopSprinting()

		healthBefore := living.Health()
		damageDealt := i.AttackDamage()
		for _, e := range p.Effects() {
			if strength, ok := e.(effect.Strength); ok {
				damageDealt += damageDealt * strength.Multiplier()
			} else if weakness, ok := e.(effect.Weakness); ok {
				damageDealt += damageDealt * weakness.Multiplier()
			}
		}

		living.Hurt(damageDealt, damage.SourceEntityAttack{Attacker: p})
		living.KnockBack(p.Position(), 0.45, 0.3608)

		if mgl64.FloatEqual(healthBefore, living.Health()) {
			p.World().PlaySound(entity.EyePosition(e), sound.Attack{})
		} else {
			p.World().PlaySound(entity.EyePosition(e), sound.Attack{Damage: true})
			p.Exhaust(0.1)
		}

		if durable, ok := i.Item().(item.Durable); ok {
			p.SetHeldItems(p.damageItem(i, durable.DurabilityInfo().AttackDurability), left)
		}
	})
}

// StartBreaking makes the player start breaking the block at the position passed using the item currently
// held in its main hand.
// If no block is present at the position, or if the block is out of range, StartBreaking will return
// immediately and the block will not be broken. StartBreaking will stop the breaking of any block that the
// player might be breaking before this method is called.
func (p *Player) StartBreaking(pos world.BlockPos, face world.Face) {
	p.AbortBreaking()
	if _, air := p.World().Block(pos).(block.Air); air || !p.canReach(pos.Vec3Centre()) {
		// The block was either out of range or air, so it can't be broken by the player.
		return
	}
	if _, ok := p.World().Block(pos.Side(face)).(block.Fire); ok {
		p.World().BreakBlockWithoutParticles(pos.Side(face))
		p.World().PlaySound(pos.Vec3(), sound.FireExtinguish{})
		return
	}
	ctx := event.C()
	p.handler().HandleStartBreak(ctx, pos)
	ctx.Continue(func() {
		if punchable, ok := p.World().Block(pos).(block.Punchable); ok {
			p.swingArm()
			punchable.Punch(pos, face, p.World(), p)
		}

		p.breaking.Store(true)
		p.breakingPos.Store(pos)

		p.swingArm()

		breakTime := p.breakTime(pos)
		for _, viewer := range p.World().Viewers(pos.Vec3()) {
			viewer.ViewBlockAction(pos, blockAction.StartCrack{BreakTime: breakTime})
		}
		p.lastBreakDuration = breakTime
	})
}

// breakTime returns the time needed to break a block at the position passed, taking into account the item
// held, if the player is on the ground/underwater and if the player has any effects.
func (p *Player) breakTime(pos world.BlockPos) time.Duration {
	held, _ := p.HeldItems()
	breakTime := block.BreakDuration(p.World().Block(pos), held)
	if !p.OnGround() {
		breakTime *= 5
	}
	if _, ok := p.World().Liquid(world.BlockPosFromVec3(p.Position().Add(mgl64.Vec3{0, p.EyeHeight()}))); ok {
		breakTime *= 5
	}
	for _, e := range p.Effects() {
		if haste, ok := e.(effect.Haste); ok {
			breakTime = time.Duration(float64(breakTime) * haste.Multiplier())
		} else if fatigue, ok := e.(effect.MiningFatigue); ok {
			breakTime = time.Duration(float64(breakTime) * fatigue.Multiplier())
		} else if conduitPower, ok := e.(effect.ConduitPower); ok {
			breakTime = time.Duration(float64(breakTime) * conduitPower.Multiplier())
		}
	}
	return breakTime
}

// FinishBreaking makes the player finish breaking the block it is currently breaking, or returns immediately
// if the player isn't breaking anything.
// FinishBreaking will stop the animation and break the block.
func (p *Player) FinishBreaking() {
	if !p.breaking.Load() {
		return
	}
	p.AbortBreaking()
	p.BreakBlock(p.breakingPos.Load().(world.BlockPos))
}

// AbortBreaking makes the player stop breaking the block it is currently breaking, or returns immediately
// if the player isn't breaking anything.
// Unlike FinishBreaking, AbortBreaking does not stop the animation.
func (p *Player) AbortBreaking() {
	if !p.breaking.CAS(true, false) {
		return
	}
	p.breakParticleCounter.Store(0)
	pos := p.breakingPos.Load().(world.BlockPos)
	for _, viewer := range p.World().Viewers(pos.Vec3()) {
		viewer.ViewBlockAction(pos, blockAction.StopCrack{})
	}
}

// ContinueBreaking makes the player continue breaking the block it started breaking after a call to
// Player.StartBreaking().
// The face passed is used to display particles on the side of the block broken.
func (p *Player) ContinueBreaking(face world.Face) {
	if !p.breaking.Load() {
		return
	}
	pos := p.breakingPos.Load().(world.BlockPos)

	p.swingArm()

	b := p.World().Block(pos)
	p.World().AddParticle(pos.Vec3(), particle.PunchBlock{Block: b, Face: face})

	if p.breakParticleCounter.Add(1)%5 == 0 {
		// We send this sound only every so often. Vanilla doesn't send it every tick while breaking
		// either. Every 5 ticks seems accurate.
		p.World().PlaySound(pos.Vec3(), sound.BlockBreaking{Block: p.World().Block(pos)})
	}
	breakTime := p.breakTime(pos)
	if breakTime != p.lastBreakDuration {
		for _, viewer := range p.World().Viewers(pos.Vec3()) {
			viewer.ViewBlockAction(pos, blockAction.ContinueCrack{BreakTime: breakTime})
		}
		p.lastBreakDuration = breakTime
	}
}

// PlaceBlock makes the player place the block passed at the position passed, granted it is within the range
// of the player.
// A use context may be passed to obtain information on if the block placement was successful. (SubCount will
// be incremented). Nil may also be passed for the context parameter.
func (p *Player) PlaceBlock(pos world.BlockPos, b world.Block, ctx *item.UseContext) {
	if p.placeBlock(pos, b, ctx.IgnoreAABB) {
		if ctx != nil {
			ctx.CountSub++
		}
	}
}

// placeBlock makes the player place the block passed at the position passed, granted it is within the range
// of the player. A bool is returned indicating if a block was placed successfully.
func (p *Player) placeBlock(pos world.BlockPos, b world.Block, ignoreAABB bool) (success bool) {
	defer func() {
		if !success {
			p.World().SetBlock(pos, p.World().Block(pos))
		}
	}()
	if !p.canReach(pos.Vec3Centre()) || !p.canEdit() {
		return false
	}
	if !ignoreAABB {
		if p.obstructedPos(pos, b) {
			return false
		}
	}

	ctx := event.C()
	p.handler().HandleBlockPlace(ctx, pos, b)
	ctx.Continue(func() {
		p.World().PlaceBlock(pos, b)
		p.World().PlaySound(pos.Vec3(), sound.BlockPlace{Block: b})
		p.swingArm()
		success = true
	})
	ctx.Stop(func() {
		pos.Neighbours(func(neighbour world.BlockPos) {
			p.World().SetBlock(neighbour, p.World().Block(neighbour))
		})
		p.World().SetBlock(pos, p.World().Block(pos))
	})
	return
}

// obstructedPos checks if the position passed is obstructed if the block passed is attempted to be placed.
// This returns true if there is an entity in the way that could prevent the block from being placed.
func (p *Player) obstructedPos(pos world.BlockPos, b world.Block) bool {
	blockBoxes := b.Model().AABB(pos, p.World())
	for i, box := range blockBoxes {
		blockBoxes[i] = box.Translate(pos.Vec3())
	}

	around := p.World().EntitiesWithin(physics.NewAABB(mgl64.Vec3{-3, -3, -3}, mgl64.Vec3{3, 3, 3}).Translate(pos.Vec3()))
	for _, e := range around {
		if _, ok := e.(*entity.Item); ok {
			// Placing blocks inside of item entities is fine.
			continue
		}
		if physics.AnyIntersections(blockBoxes, e.AABB().Translate(e.Position())) {
			return true
		}
	}
	return false
}

// BreakBlock makes the player break a block in the world at a position passed. If the player is unable to
// reach the block passed, the method returns immediately.
func (p *Player) BreakBlock(pos world.BlockPos) {
	if !p.canReach(pos.Vec3Centre()) || !p.canEdit() {
		return
	}
	b := p.World().Block(pos)
	if _, air := b.(block.Air); air {
		// Don't do anything if the position broken is already air.
		return
	}
	if _, breakable := b.(block.Breakable); !breakable && p.survival() {
		// Block cannot be broken server-side. Set the block back so viewers have it resent and cancel all
		// further action.
		p.World().SetBlock(pos, p.World().Block(pos))
		return
	}

	ctx := event.C()
	p.handler().HandleBlockBreak(ctx, pos)

	ctx.Continue(func() {
		p.swingArm()
		p.World().BreakBlock(pos)
		held, left := p.HeldItems()

		for _, drop := range p.drops(held, b) {
			itemEntity := entity.NewItem(drop, pos.Vec3Centre())
			itemEntity.SetVelocity(mgl64.Vec3{rand.Float64()*0.2 - 0.1, 0.2, rand.Float64()*0.2 - 0.1})
			p.World().AddEntity(itemEntity)
		}

		p.Exhaust(0.005)

		if !block.BreaksInstantly(b, held) {
			if durable, ok := held.Item().(item.Durable); ok {
				p.SetHeldItems(p.damageItem(held, durable.DurabilityInfo().BreakDurability), left)
			}
		}
	})
	ctx.Stop(func() {
		p.World().SetBlock(pos, p.World().Block(pos))
	})
}

// drops returns the drops that the player can get from the block passed using the item held.
func (p *Player) drops(held item.Stack, b world.Block) []item.Stack {
	t, ok := held.Item().(tool.Tool)
	if !ok {
		t = tool.None{}
	}
	var drops []item.Stack
	if container, ok := b.(block.Container); ok {
		// If the block is a container, it should drop its inventory contents regardless whether the
		// player is in creative mode or not.
		drops = container.Inventory().Contents()
		if breakable, ok := b.(block.Breakable); ok && p.survival() {
			if breakable.BreakInfo().Harvestable(t) {
				drops = breakable.BreakInfo().Drops(t)
			}
		}
		container.Inventory().Clear()
	} else if breakable, ok := b.(block.Breakable); ok && p.survival() {
		if breakable.BreakInfo().Harvestable(t) {
			drops = breakable.BreakInfo().Drops(t)
		}
	} else if it, ok := b.(world.Item); ok && p.survival() {
		drops = []item.Stack{item.NewStack(it, 1)}
	}
	return drops
}

// PickBlock makes the player pick a block in the world at a position passed. If the player is unable to
// pick the block, the method returns immediately.
func (p *Player) PickBlock(pos world.BlockPos) {
	if !p.canReach(pos.Vec3()) {
		return
	}

	b := p.World().Block(pos)
	if i, ok := b.(world.Item); ok {
		copiedItem := item.NewStack(i, 1)
		copiedItem = nbtconv.ItemFromNBT(nbtconv.ItemToNBT(copiedItem, false), nil)

		slot, found := p.Inventory().First(copiedItem)

		if (!found && p.GameMode() != gamemode.Creative{}) {
			return
		}

		ctx := event.C()
		p.handler().HandleBlockPick(ctx, pos, b)

		ctx.Continue(func() {
			_, offhand := p.HeldItems()

			if found {
				if slot < 9 {
					_ = p.session().SetHeldSlot(slot)
				} else {
					_ = p.Inventory().Swap(slot, int(p.heldSlot.Load()))
				}
			} else {
				firstEmpty, emptyFound := p.Inventory().FirstEmpty()

				if !emptyFound {
					p.SetHeldItems(copiedItem, offhand)
				} else if firstEmpty < 8 {
					_ = p.session().SetHeldSlot(firstEmpty)
					_ = p.Inventory().SetItem(firstEmpty, copiedItem)
				} else {
					_ = p.Inventory().Swap(firstEmpty, int(p.heldSlot.Load()))
					p.SetHeldItems(copiedItem, offhand)
				}
			}
		})
	}
}

// Teleport teleports the player to a target position in the world. Unlike Move, it immediately changes the
// position of the player, rather than showing an animation.
func (p *Player) Teleport(pos mgl64.Vec3) {
	// Generally it is expected you are teleported to the middle of the block.
	pos = pos.Add(mgl64.Vec3{0.5, 0, 0.5})

	ctx := event.C()
	p.handler().HandleTeleport(ctx, pos)
	ctx.Continue(func() {
		p.teleport(pos)
	})
}

// teleport teleports the player to a target position in the world. It does not call the handler of the
// player.
func (p *Player) teleport(pos mgl64.Vec3) {
	p.session().ViewEntityTeleport(p, pos)
	for _, v := range p.World().Viewers(p.Position()) {
		v.ViewEntityTeleport(p, pos)
	}
	p.pos.Store(pos)
}

// Move moves the player from one position to another in the world, by adding the delta passed to the current
// position of the player.
func (p *Player) Move(deltaPos mgl64.Vec3) {
	if p.Dead() || p.immobile.Load() || deltaPos.ApproxEqual(mgl64.Vec3{}) {
		return
	}

	ctx := event.C()
	p.handler().HandleMove(ctx, p.Position().Add(deltaPos), p.Yaw(), p.Pitch())
	ctx.Continue(func() {
		for _, v := range p.World().Viewers(p.Position()) {
			v.ViewEntityMovement(p, deltaPos, 0, 0)
		}
		p.pos.Store(p.Position().Add(deltaPos))

		// The vertical axis isn't relevant for calculation of exhaustion points.
		deltaPos[1] = 0
		if p.Swimming() {
			p.Exhaust(0.01 * deltaPos.Len())
		} else if p.Sprinting() {
			p.Exhaust(0.1 * deltaPos.Len())
		}
	})
	ctx.Stop(func() {
		p.teleport(p.Position())
	})
}

// Rotate rotates the player, adding deltaYaw and deltaPitch to the respective values.
func (p *Player) Rotate(deltaYaw, deltaPitch float64) {
	if p.Dead() || (mgl64.FloatEqual(deltaYaw, 0) && mgl64.FloatEqual(deltaPitch, 0)) {
		return
	}

	p.handler().HandleMove(event.C(), p.Position(), p.Yaw()+deltaYaw, p.Pitch()+deltaPitch)

	// Cancelling player rotation is rather scuffed, so we don't do that.
	for _, v := range p.World().Viewers(p.Position()) {
		v.ViewEntityMovement(p, mgl64.Vec3{}, deltaYaw, deltaPitch)
	}
	p.yaw.Store(p.Yaw() + deltaYaw)
	p.pitch.Store(p.Pitch() + deltaPitch)
}

// Facing returns the horizontal direction that the player is facing.
func (p *Player) Facing() world.Direction {
	return entity.Facing(p)
}

// World returns the world that the player is currently in.
func (p *Player) World() *world.World {
	w, _ := world.OfEntity(p)
	return w
}

// Position returns the current position of the player. It may be changed as the player moves or is moved
// around the world.
func (p *Player) Position() mgl64.Vec3 {
	return p.pos.Load().(mgl64.Vec3)
}

// Yaw returns the yaw of the entity. This is horizontal rotation (rotation around the vertical axis), and
// is 0 when the entity faces forward.
func (p *Player) Yaw() float64 {
	return p.yaw.Load()
}

// Pitch returns the pitch of the entity. This is vertical rotation (rotation around the horizontal axis),
// and is 0 when the entity faces forward.
func (p *Player) Pitch() float64 {
	return p.pitch.Load()
}

// Collect makes the player collect the item stack passed, adding it to the inventory.
func (p *Player) Collect(s item.Stack) (n int) {
	ctx := event.C()
	p.handler().HandleItemPickup(ctx, s)
	ctx.Continue(func() {
		n, _ = p.Inventory().AddItem(s)
	})
	return
}

// Drop makes the player drop the item.Stack passed as an entity.Item, so that it may be picked up from the
// ground.
// The dropped item entity has a pickup delay of 2 seconds.
// The number of items that was dropped in the end is returned. It is generally the count of the stack passed
// or 0 if dropping the item.Stack was cancelled.
func (p *Player) Drop(s item.Stack) (n int) {
	e := entity.NewItem(s, p.Position().Add(mgl64.Vec3{0, 1.4}))
	e.SetVelocity(entity.DirectionVector(p).Mul(0.4))
	e.SetPickupDelay(time.Second * 2)

	ctx := event.C()
	p.handler().HandleItemDrop(ctx, e)

	ctx.Continue(func() {
		p.World().AddEntity(e)
		n = s.Count()
	})
	return
}

// OpenBlockContainer opens a block container, such as a chest, at the position passed. If no container was
// present at that location, OpenBlockContainer does nothing.
// OpenBlockContainer will also do nothing if the player has no session connected to it.
func (p *Player) OpenBlockContainer(pos world.BlockPos) {
	if p.session() == session.Nop {
		return
	}
	p.session().OpenBlockContainer(pos)
}

// Latency returns a rolling average of latency between the sending and the receiving end of the connection of
// the player.
// The latency returned is updated continuously and is half the round trip time (RTT).
// If the Player does not have a session associated with it, Latency returns 0.
func (p *Player) Latency() time.Duration {
	if p.session() == session.Nop {
		return 0
	}
	return p.session().Latency()
}

// Tick ticks the entity, performing actions such as checking if the player is still breaking a block.
func (p *Player) Tick(current int64) {
	if p.Dead() {
		return
	}
	if _, ok := p.World().Liquid(world.BlockPosFromVec3(p.Position())); !ok {
		p.StopSwimming()
	}
	p.onGround.Store(p.checkOnGround())
	p.tickFood()
	p.effects.Tick(p)
	if p.Position()[1] < 0 && p.survival() && current%10 == 0 {
		p.Hurt(4, damage.SourceVoid{})
	}

	if p.OnFireDuration() > 0 {
		p.fireTicks.Sub(1)
		if p.FireProof() || p.OnFireDuration() <= 0 {
			p.Extinguish()
		}
		if p.OnFireDuration()%time.Second == 0 && !p.AttackImmune() {
			p.Hurt(1, damage.SourceFireTick{})
		}
	}

	// TODO: Move to Move()
	aabb := p.AABB().Translate(p.Position())
	min, max := world.BlockPosFromVec3(aabb.Min()), world.BlockPosFromVec3(aabb.Max())
	for x := min[0]; x <= max[0]; x++ {
		for y := min[1]; y <= max[1]; y++ {
			for z := min[2]; z <= max[2]; z++ {
				if collide, ok := p.World().Block(world.BlockPos{x, y, z}).(block.EntityCollider); ok {
					collide.EntityCollide(p)
				}
			}
		}
	}

	if current%4 == 0 && p.usingItem.Load() {
		held, _ := p.HeldItems()
		if _, ok := held.Item().(item.Consumable); ok {
			// Eating particles seem to happen roughly every 4 ticks.
			for _, v := range p.World().Viewers(p.Position()) {
				v.ViewEntityAction(p, action.Eat{})
			}
		}
	}
}

// tickFood ticks food related functionality, such as the depletion of the food bar and regeneration if it
// is full enough.
func (p *Player) tickFood() {
	p.hunger.foodTick++
	if p.hunger.foodTick == 10 && (p.hunger.canQuicklyRegenerate() || p.World().Difficulty() == difficulty.Peaceful{}) {
		p.hunger.foodTick = 0
		p.regenerate()
		if (p.World().Difficulty() == difficulty.Peaceful{}) {
			p.AddFood(1)
		}
	} else if p.hunger.foodTick == 80 {
		p.hunger.foodTick = 0
		if p.hunger.canRegenerate() {
			p.regenerate()
		} else if p.hunger.starving() {
			p.starve()
		}
	}
}

// regenerate attempts to regenerate half a heart of health, typically caused by a full food bar.
func (p *Player) regenerate() {
	if p.Health() == p.MaxHealth() {
		return
	}
	p.Heal(1, healing.SourceFood{})
	p.Exhaust(6)
}

// starve deals starvation damage to the player if the difficult allows it. In peaceful mode, no damage will
// ever be dealt. In easy mode, damage will only be dealt if the player has more than 10 health. In normal
// mode, damage will only be dealt if the player has more than 2 health and in hard mode, damage will always
// be dealt.
func (p *Player) starve() {
	switch p.World().Difficulty().(type) {
	case difficulty.Peaceful:
		return
	case difficulty.Easy:
		if p.Health() <= 10 {
			return
		}
	case difficulty.Normal:
		if p.Health() <= 2 {
			return
		}
	}
	p.Hurt(1, damage.SourceStarvation{})
}

// checkOnGround checks if the player is currently considered to be on the ground.
func (p *Player) checkOnGround() bool {
	pos := p.Position()
	pAABB := p.AABB().Translate(pos)
	min, max := pAABB.Min(), pAABB.Max()

	for x := min[0]; x <= max[0]+1; x++ {
		for z := min[2]; z <= max[2]+1; z++ {
			for y := pos[1] - 1; y < pos[1]+1; y++ {
				bPos := world.BlockPosFromVec3(mgl64.Vec3{x, y, z})
				b := p.World().Block(bPos)
				aabbList := b.Model().AABB(bPos, p.World())
				for _, aabb := range aabbList {
					if aabb.GrowVec3(mgl64.Vec3{0, 0.05}).Translate(bPos.Vec3()).IntersectsWith(pAABB) {
						return true
					}
				}
			}
		}
	}
	return false
}

// Velocity returns the current velocity of the player.
func (p *Player) Velocity() mgl64.Vec3 {
	// TODO: Implement server-side movement of player entities.
	return p.velocity.Load().(mgl64.Vec3)
}

// SetVelocity sets the velocity of the player.
func (p *Player) SetVelocity(v mgl64.Vec3) {
	// TODO: Implement server-side movement of player entities.
	p.velocity.Store(v)
}

// AABB returns the axis aligned bounding box of the player.
func (p *Player) AABB() physics.AABB {
	switch {
	case p.Sneaking():
		return physics.NewAABB(mgl64.Vec3{-0.3, 0, -0.3}, mgl64.Vec3{0.3, 1.65, 0.3})
	case p.Swimming():
		return physics.NewAABB(mgl64.Vec3{-0.3, 0, -0.3}, mgl64.Vec3{0.3, 0.6, 0.3})
	default:
		return physics.NewAABB(mgl64.Vec3{-0.3, 0, -0.3}, mgl64.Vec3{0.3, 1.8, 0.3})
	}
}

// OnGround checks if the player is considered to be on the ground.
func (p *Player) OnGround() bool {
	return p.onGround.Load()
}

// EyeHeight returns the eye height of the player: 1.62, or 0.52 if the player is swimming.
func (p *Player) EyeHeight() float64 {
	if p.swimming.Load() {
		return 0.52
	}
	return 1.62
}

// PlaySound plays a world.Sound that only this Player can hear. Unlike World.PlaySound, it is not broadcast
// to players around it.
func (p *Player) PlaySound(sound world.Sound) {
	p.session().ViewSound(p.Position().Add(mgl64.Vec3{0, p.EyeHeight()}), sound)
}

// CanFly returns whether the client can fly.
func (p *Player) CanFly() bool {
	return p.session().CanFly()
}

// SetCanFly sets whether or not the client can fly.
func (p *Player) SetCanFly(canfly bool) {
	p.session().SetCanFly(canfly)
}

// Flying returns whether the client is currently flying or not.
func (p *Player) Flying() bool {
	return p.session().Flying()
}

// SetFlying sets whether or not the client is currently flying.
func (p *Player) SetFlying(flying bool) {
	p.session().SetFlying(flying)
}

// State returns the current state of the player. Types from the `entity/state` package are returned
// depending on what the player is currently doing.
func (p *Player) State() (s []state.State) {
	if p.Sneaking() {
		s = append(s, state.Sneaking{})
	}
	if p.Sprinting() {
		s = append(s, state.Sprinting{})
	}
	if p.Swimming() {
		s = append(s, state.Swimming{})
	}
	if p.canBreathe() || !p.survival() {
		s = append(s, state.Breathing{})
	}
	if p.invisible.Load() {
		s = append(s, state.Invisible{})
	}
	if p.immobile.Load() {
		s = append(s, state.Immobile{})
	}
	if p.usingItem.Load() {
		s = append(s, state.UsingItem{})
	}
	if p.OnFireDuration() > 0 {
		s = append(s, state.OnFire{})
	}
	s = append(s, state.Scale{Scale: p.scale.Load()})
	colour, ambient := effect.ResultingColour(p.Effects())
	if (colour != color.RGBA{}) {
		s = append(s, state.EffectBearing{ParticleColour: colour, Ambient: ambient})
	}
	s = append(s, state.Named{NameTag: p.nameTag.Load()})
	return
}

// updateState updates the state of the player to all viewers of the player.
func (p *Player) updateState() {
	for _, v := range p.World().Viewers(p.Position()) {
		v.ViewEntityState(p, p.State())
	}
}

// canBreathe checks if the player is currently able to breathe. If it's underwater and the player does not
// have the water breathing or conduit power effect, this returns false.
func (p *Player) canBreathe() bool {
	for _, e := range p.Effects() {
		if _, waterBreathing := e.(effect.WaterBreathing); waterBreathing {
			return true
		}
		if _, conduitPower := e.(effect.ConduitPower); conduitPower {
			return true
		}
	}
	_, submerged := p.World().Liquid(world.BlockPosFromVec3(p.Position().Add(mgl64.Vec3{0, p.EyeHeight()})))
	return !submerged
}

// swingArm makes the player swing its arm.
func (p *Player) swingArm() {
	if p.Dead() {
		return
	}
	for _, v := range p.World().Viewers(p.Position()) {
		v.ViewEntityAction(p, action.SwingArm{})
	}
}

// EncodeEntity ...
func (p *Player) EncodeEntity() string {
	return "minecraft:player"
}

// Close closes the player and removes it from the world.
// Close disconnects the player with a 'Connection closed.' message. Disconnect should be used to disconnect a
// player with a custom message.
func (p *Player) Close() error {
	if p.World() == nil {
		return nil
	}
	p.session().Disconnect("Connection closed.")
	p.close()
	return nil
}

// damageItem damages the item stack passed with the damage passed and returns the new stack. If the item
// broke, a breaking sound is played.
// If the player is not survival, the original stack is returned.
func (p *Player) damageItem(s item.Stack, d int) item.Stack {
	if !p.survival() || d == 0 {
		return s
	}
	ctx := event.C()
	p.handler().HandleItemDamage(ctx, s, d)

	ctx.Continue(func() {
		s = s.Damage(d)
		if s.Empty() {
			p.World().PlaySound(p.Position(), sound.ItemBreak{})
		}
	})
	return s
}

// subtractItem subtracts d from the count of the item stack passed and returns it, if the player is in
// survival or adventure mode.
func (p *Player) subtractItem(s item.Stack, d int) item.Stack {
	if p.survival() && d != 0 {
		return s.Grow(-d)
	}
	return s
}

// addNewItem adds the new item of the context passed to the inventory.
func (p *Player) addNewItem(ctx *item.UseContext) {
	if (ctx.NewItemSurvivalOnly && !p.survival()) || ctx.NewItem.Empty() {
		return
	}
	held, left := p.HeldItems()
	if held.Empty() {
		p.SetHeldItems(ctx.NewItem, left)
		return
	}
	n, err := p.Inventory().AddItem(ctx.NewItem)
	if err != nil {
		// Not all items could be added to the inventory, so drop the rest.
		p.Drop(ctx.NewItem.Grow(ctx.NewItem.Count() - n))
	}
}

// canReach checks if a player can reach a position with its current range. The range depends on if the player
// is either survival or creative mode.
func (p *Player) canReach(pos mgl64.Vec3) bool {
	const (
		eyeHeight     = 1.62
		creativeRange = 13.0
		survivalRange = 7.0
	)
	if (p.GameMode() == gamemode.Spectator{}) {
		return false
	}
	eyes := p.Position().Add(mgl64.Vec3{0, eyeHeight})

	if (p.GameMode() == gamemode.Creative{}) {
		return world.Distance(eyes, pos) <= creativeRange && !p.Dead()
	}
	return world.Distance(eyes, pos) <= survivalRange && !p.Dead()
}

// close closed the player without disconnecting it. It executes code shared by both the closing and the
// disconnecting of players.
func (p *Player) close() {
	p.handler().HandleQuit()

	p.Handle(NopHandler{})
	chat.Global.Unsubscribe(p)

	p.sMutex.Lock()
	s := p.s
	p.s = nil
	p.sMutex.Unlock()

	// Clear the inventories so that they no longer hold references to the connection.
	_ = p.inv.Close()
	_ = p.offHand.Close()
	_ = p.armour.Close()

	if p.World() == nil {
		return
	}

	if s == nil {
		p.World().RemoveEntity(p)
	} else {
		s.CloseConnection()
	}
}

// session returns the network session of the player. If it has one, it is returned. If not, a no-op session
// is returned.
func (p *Player) session() *session.Session {
	p.sMutex.RLock()
	s := p.s
	p.sMutex.RUnlock()

	if s == nil {
		return session.Nop
	}
	return s
}

// handler returns the handler of the player.
func (p *Player) handler() Handler {
	p.hMutex.RLock()
	handler := p.h
	p.hMutex.RUnlock()
	return handler
}

// broadcastItems broadcasts the items held to viewers.
func (p *Player) broadcastItems(int, item.Stack) {
	for _, viewer := range p.World().Viewers(p.Position()) {
		viewer.ViewEntityItems(p)
	}
}

// broadcastArmour broadcasts the armour equipped to viewers.
func (p *Player) broadcastArmour(int, item.Stack) {
	for _, viewer := range p.World().Viewers(p.Position()) {
		viewer.ViewEntityArmour(p)
	}
}

// format is a utility function to format a list of values to have spaces between them, but no newline at the
// end, which is typically used for sending messages, popups and tips.
func format(a []interface{}) string {
	return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintln(a...), "\n"), "\n")
}
