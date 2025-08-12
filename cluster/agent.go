package cluster

import (
	"log/slog"
	"reflect"
	"strings"

	"github.com/khulnasoft/goactors/actor"
	"golang.org/x/exp/maps"
)

// Internal cluster messages
type (
	activate struct {
		kind   string
		config ActivationConfig
	}
	getMembers struct{}
	getKinds   struct{}
	deactivate struct{ pid *actor.PID }
	getActive  struct {
		id   string
		kind string
	}
)

// Agent manages the state and membership of the cluster.
type Agent struct {
	members    *MemberSet
	cluster    *Cluster
	kinds      map[string]bool
	localKinds map[string]kind
	activated  map[string]*actor.PID // cluster-wide activated actors
}

// NewAgent returns a producer that creates a new Agent actor.
func NewAgent(c *Cluster) actor.Producer {
	kinds := make(map[string]bool)
	localKinds := make(map[string]kind)
	for _, k := range c.kinds {
		kinds[k.name] = true
		localKinds[k.name] = k
	}
	return func() actor.Receiver {
		return &Agent{
			members:    NewMemberSet(),
			cluster:    c,
			kinds:      kinds,
			localKinds: localKinds,
			activated:  make(map[string]*actor.PID),
		}
	}
}

// Receive handles all messages sent to the Agent actor.
func (a *Agent) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		// Optionally log startup or do initialization here.
	case actor.Stopped:
		// Optionally log shutdown or do cleanup here.
	case *ActorTopology:
		a.handleActorTopology(msg)
	case *Members:
		a.handleMembers(msg.Members)
	case *Activation:
		a.handleActivation(msg)
	case activate:
		pid := a.activate(msg.kind, msg.config)
		c.Respond(pid)
	case deactivate:
		a.bcast(&Deactivation{PID: msg.pid})
	case *Deactivation:
		a.handleDeactivation(msg)
	case *ActivationRequest:
		resp := a.handleActivationRequest(msg)
		c.Respond(resp)
	case getMembers:
		c.Respond(a.members.Slice())
	case getKinds:
		c.Respond(maps.Keys(a.kinds))
	case getActive:
		a.handleGetActive(c, msg)
	}
}

// handleGetActive responds with activated actor(s) by id or kind.
func (a *Agent) handleGetActive(c *actor.Context, msg getActive) {
	if msg.id != "" {
		c.Respond(a.activated[msg.id])
		return
	}
	if msg.kind != "" {
		var pids []*actor.PID
		for id, pid := range a.activated {
			if parts := strings.Split(id, "/"); len(parts) == 2 && parts[0] == msg.kind {
				pids = append(pids, pid)
			}
		}
		c.Respond(pids)
	}
}

// handleActorTopology adds activated actors from topology update.
func (a *Agent) handleActorTopology(msg *ActorTopology) {
	for _, info := range msg.Actors {
		a.addActivated(info.PID)
	}
}

// handleDeactivation removes actor and broadcasts event.
func (a *Agent) handleDeactivation(msg *Deactivation) {
	a.removeActivated(msg.PID)
	a.cluster.engine.Poison(msg.PID)
	a.cluster.engine.BroadcastEvent(DeactivationEvent{PID: msg.PID})
}

// handleActivation adds actor and broadcasts event.
func (a *Agent) handleActivation(msg *Activation) {
	a.addActivated(msg.PID)
	a.cluster.engine.BroadcastEvent(ActivationEvent{PID: msg.PID})
}

// handleActivationRequest spawns actor if kind exists locally.
func (a *Agent) handleActivationRequest(msg *ActivationRequest) *ActivationResponse {
	if !a.hasKindLocal(msg.Kind) {
		slog.Error("activation request: kind not registered locally", "kind", msg.Kind)
		return &ActivationResponse{Success: false}
	}
	kind := a.localKinds[msg.Kind]
	pid := a.cluster.engine.Spawn(kind.producer, msg.Kind, actor.WithID(msg.ID))
	return &ActivationResponse{PID: pid, Success: true}
}

// activate starts an actor on the correct member and broadcasts activation.
func (a *Agent) activate(kind string, config ActivationConfig) *actor.PID {
	id := kind + "/" + config.id
	if _, exists := a.activated[id]; exists {
		slog.Warn("duplicate actor id across cluster", "id", id)
		return nil
	}
	members := a.members.FilterByKind(kind)
	if len(members) == 0 {
		slog.Warn("no members with kind found", "kind", kind)
		return nil
	}
	if config.selectMember == nil {
		config.selectMember = SelectRandomMember
	}
	member := config.selectMember(ActivationDetails{
		Members: members,
		Region:  config.region,
		Kind:    kind,
	})
	if member == nil {
		slog.Warn("no member selected for activation")
		return nil
	}
	req := &ActivationRequest{Kind: kind, ID: config.id}
	activatorPID := actor.NewPID(member.Host, "cluster/"+member.ID)

	var resp *ActivationResponse
	if member.Host == a.cluster.engine.Address() {
		resp = a.handleActivationRequest(req)
	} else {
		result, err := a.cluster.engine.Request(activatorPID, req, a.cluster.config.requestTimeout).Result()
		if err != nil {
			slog.Error("activation request failed", "err", err)
			return nil
		}
		r, ok := result.(*ActivationResponse)
		if !ok || !r.Success {
			slog.Error("activation unsuccessful", "response", r)
			return nil
		}
		resp = r
	}
	a.bcast(&Activation{PID: resp.PID})
	return resp.PID
}

// handleMembers updates cluster membership and broadcasts changes.
func (a *Agent) handleMembers(members []*Member) {
	joined := NewMemberSet(members...).Except(a.members.Slice())
	left := a.members.Except(members)

	for _, member := range joined {
		a.memberJoin(member)
	}
	for _, member := range left {
		a.memberLeave(member)
	}
}

// memberJoin adds member and broadcasts join event and topology.
func (a *Agent) memberJoin(member *Member) {
	a.members.Add(member)
	for _, kind := range member.Kinds {
		a.kinds[kind] = true
	}
	actorInfos := make([]*ActorInfo, 0, len(a.activated))
	for _, pid := range a.activated {
		actorInfos = append(actorInfos, &ActorInfo{PID: pid})
	}
	if len(actorInfos) > 0 {
		a.cluster.engine.Send(member.PID(), &ActorTopology{Actors: actorInfos})
	}
	a.cluster.engine.BroadcastEvent(MemberJoinEvent{Member: member})
	slog.Debug("[CLUSTER] member joined", "id", member.ID, "host", member.Host, "kinds", member.Kinds, "region", member.Region)
}

// memberLeave removes member and updates available kinds.
func (a *Agent) memberLeave(member *Member) {
	a.members.Remove(member)
	a.rebuildKinds()
	for _, pid := range a.activated {
		if pid.Address == member.Host {
			a.removeActivated(pid)
		}
	}
	a.cluster.engine.BroadcastEvent(MemberLeaveEvent{Member: member})
	slog.Debug("[CLUSTER] member left", "id", member.ID, "host", member.Host, "kinds", member.Kinds)
}

// bcast sends a message to all members of the cluster.
func (a *Agent) bcast(msg any) {
	a.members.ForEach(func(member *Member) bool {
		a.cluster.engine.Send(member.PID(), msg)
		return true
	})
}

// addActivated tracks a new activated actor.
func (a *Agent) addActivated(pid *actor.PID) {
	if _, exists := a.activated[pid.ID]; !exists {
		a.activated[pid.ID] = pid
		slog.Debug("actor activated on cluster", "pid", pid)
	}
}

// removeActivated stops tracking an actor.
func (a *Agent) removeActivated(pid *actor.PID) {
	delete(a.activated, pid.ID)
	slog.Debug("actor removed from cluster", "pid", pid)
}

// hasKindLocal returns true if the kind is available locally.
func (a *Agent) hasKindLocal(name string) bool {
	_, exists := a.localKinds[name]
	return exists
}

// rebuildKinds updates the global kinds map after membership changes.
func (a *Agent) rebuildKinds() {
	maps.Clear(a.kinds)
	a.members.ForEach(func(m *Member) bool {
		for _, kind := range m.Kinds {
			a.kinds[kind] = true
		}
		return true
	})
}
