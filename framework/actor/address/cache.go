package address

import (
	"time"

	"github.com/GooLuck/GoServer/framework/service_discovery"
)

// 缓存非本节点actor地址

type PlayerAddressCache struct {
	localCache    map[string]Address   // playerID -> address
	ttlCache      map[string]time.Time // 过期时间
	discovery     service_discovery.Registry
	currentNodeID string
}

func (pac *PlayerAddressCache) GetAddress(playerID string) (Address, error) {
	// 1. 检查本地缓存
	if addr, exists := pac.localCache[playerID]; exists {
		if time.Since(pac.ttlCache[playerID]) < 5*time.Minute {
			return addr, nil
		}
	}

	// // 2. 检查本地actor
	// actorMgr := cluster.GetDefaultClusterManager().GetDefaultActorManager()
	// if actorMgr.HasActor(playerID) {
	// 	addr := message.NewLocalActorAddress(fmt.Sprintf("/players/%s", playerID))
	// 	pac.updateCache(playerID, addr)
	// 	return addr, nil
	// }

	// // 3. 查询etcd
	// services, err := pac.discovery.Discover(ctx, "player-actor")
	// // ... 找到并缓存

	// // 4. 构建地址
	// if service.Metadata["node_id"] == pac.currentNodeID {
	// 	addr = message.NewLocalActorAddress(fmt.Sprintf("/players/%s", playerID))
	// } else {
	// 	addr = message.ParseAddress(service.Address)
	// }

	// pac.updateCache(playerID, addr)
	return nil, nil
}
