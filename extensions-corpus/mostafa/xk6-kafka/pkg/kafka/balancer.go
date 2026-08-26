package kafka

type BalancerKeyFunc func(key []byte, partitions ...int) (partition int)
