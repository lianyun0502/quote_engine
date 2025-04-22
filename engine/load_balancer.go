package quote_engine

type LoadBalancer struct {
    wsSubscriptions map[int]int // WebSocket ID -> 訂閱數量
}

func NewLoadBalancer(poolSize int) *LoadBalancer {
    wsSubscriptions := make(map[int]int)
    for i := 0; i < poolSize; i++ {
        wsSubscriptions[i] = 0
    }
    return &LoadBalancer{
        wsSubscriptions: wsSubscriptions,
    }
}

func (lb *LoadBalancer) AddSubscription(wsID int) {
    lb.wsSubscriptions[wsID]++
}

func (lb *LoadBalancer) RemoveSubscription(wsID int) {
    if lb.wsSubscriptions[wsID] > 0 {
        lb.wsSubscriptions[wsID]--
    }
}

func (lb *LoadBalancer) GetLeastLoadedWS() int {
    minSubscriptions := int(^uint(0) >> 1) // Max int value
    var wsID int
    for id, count := range lb.wsSubscriptions {
        if count < minSubscriptions {
            minSubscriptions = count
            wsID = id
        }
    }
    return wsID
}