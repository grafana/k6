package redis

import (
	"context"
	"encoding"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/redis/go-redis/v9/internal"
)

// KeepTTL is a Redis KEEPTTL option to keep existing TTL, it requires your redis-server version >= 6.0,
// otherwise you will receive an error: (error) ERR syntax error.
// For example:
//
//	rdb.Set(ctx, key, value, redis.KeepTTL)
const KeepTTL = -1

func usePrecise(dur time.Duration) bool {
	return dur < time.Second || dur%time.Second != 0
}

func formatMs(ctx context.Context, dur time.Duration) int64 {
	if dur > 0 && dur < time.Millisecond {
		internal.Logger.Printf(
			ctx,
			"specified duration is %s, but minimal supported value is %s - truncating to 1ms",
			dur, time.Millisecond,
		)
		return 1
	}
	return int64(dur / time.Millisecond)
}

func formatSec(ctx context.Context, dur time.Duration) int64 {
	if dur > 0 && dur < time.Second {
		internal.Logger.Printf(
			ctx,
			"specified duration is %s, but minimal supported value is %s - truncating to 1s",
			dur, time.Second,
		)
		return 1
	}
	return int64(dur / time.Second)
}

func appendArgs(dst, src []interface{}) []interface{} {
	if len(src) == 1 {
		return appendArg(dst, src[0])
	}

	dst = append(dst, src...)
	return dst
}

func appendArg(dst []interface{}, arg interface{}) []interface{} {
	switch arg := arg.(type) {
	case []string:
		for _, s := range arg {
			dst = append(dst, s)
		}
		return dst
	case []interface{}:
		dst = append(dst, arg...)
		return dst
	case map[string]interface{}:
		for k, v := range arg {
			dst = append(dst, k, v)
		}
		return dst
	case map[string]string:
		for k, v := range arg {
			dst = append(dst, k, v)
		}
		return dst
	case time.Time, time.Duration, encoding.BinaryMarshaler, net.IP:
		return append(dst, arg)
	default:
		// scan struct field
		v := reflect.ValueOf(arg)
		if v.Type().Kind() == reflect.Ptr {
			if v.IsNil() {
				// error: arg is not a valid object
				return dst
			}
			v = v.Elem()
		}

		if v.Type().Kind() == reflect.Struct {
			return appendStructField(dst, v)
		}

		return append(dst, arg)
	}
}

// appendStructField appends the field and value held by the structure v to dst, and returns the appended dst.
func appendStructField(dst []interface{}, v reflect.Value) []interface{} {
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("redis")
		if tag == "" || tag == "-" {
			continue
		}
		name, opt, _ := strings.Cut(tag, ",")
		if name == "" {
			continue
		}

		field := v.Field(i)

		// miss field
		if omitEmpty(opt) && isEmptyValue(field) {
			continue
		}

		if field.CanInterface() {
			dst = append(dst, name, field.Interface())
		}
	}

	return dst
}

func omitEmpty(opt string) bool {
	for opt != "" {
		var name string
		name, opt, _ = strings.Cut(opt, ",")
		if name == "omitempty" {
			return true
		}
	}
	return false
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

type Cmdable interface {
	Pipeline() Pipeliner
	Pipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error)

	TxPipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error)
	TxPipeline() Pipeliner

	Command(ctx context.Context) *CommandsInfoCmd
	CommandList(ctx context.Context, filter *FilterBy) *StringSliceCmd
	CommandGetKeys(ctx context.Context, commands ...interface{}) *StringSliceCmd
	CommandGetKeysAndFlags(ctx context.Context, commands ...interface{}) *KeyFlagsCmd
	ClientGetName(ctx context.Context) *StringCmd
	Echo(ctx context.Context, message interface{}) *StringCmd
	Ping(ctx context.Context) *StatusCmd
	Quit(ctx context.Context) *StatusCmd
	Del(ctx context.Context, keys ...string) *IntCmd
	Unlink(ctx context.Context, keys ...string) *IntCmd
	Dump(ctx context.Context, key string) *StringCmd
	Exists(ctx context.Context, keys ...string) *IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	ExpireAt(ctx context.Context, key string, tm time.Time) *BoolCmd
	ExpireTime(ctx context.Context, key string) *DurationCmd
	ExpireNX(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	ExpireXX(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	ExpireGT(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	ExpireLT(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	Keys(ctx context.Context, pattern string) *StringSliceCmd
	Migrate(ctx context.Context, host, port, key string, db int, timeout time.Duration) *StatusCmd
	Move(ctx context.Context, key string, db int) *BoolCmd
	ObjectRefCount(ctx context.Context, key string) *IntCmd
	ObjectEncoding(ctx context.Context, key string) *StringCmd
	ObjectIdleTime(ctx context.Context, key string) *DurationCmd
	Persist(ctx context.Context, key string) *BoolCmd
	PExpire(ctx context.Context, key string, expiration time.Duration) *BoolCmd
	PExpireAt(ctx context.Context, key string, tm time.Time) *BoolCmd
	PExpireTime(ctx context.Context, key string) *DurationCmd
	PTTL(ctx context.Context, key string) *DurationCmd
	RandomKey(ctx context.Context) *StringCmd
	Rename(ctx context.Context, key, newkey string) *StatusCmd
	RenameNX(ctx context.Context, key, newkey string) *BoolCmd
	Restore(ctx context.Context, key string, ttl time.Duration, value string) *StatusCmd
	RestoreReplace(ctx context.Context, key string, ttl time.Duration, value string) *StatusCmd
	Sort(ctx context.Context, key string, sort *Sort) *StringSliceCmd
	SortRO(ctx context.Context, key string, sort *Sort) *StringSliceCmd
	SortStore(ctx context.Context, key, store string, sort *Sort) *IntCmd
	SortInterfaces(ctx context.Context, key string, sort *Sort) *SliceCmd
	Touch(ctx context.Context, keys ...string) *IntCmd
	TTL(ctx context.Context, key string) *DurationCmd
	Type(ctx context.Context, key string) *StatusCmd
	Append(ctx context.Context, key, value string) *IntCmd
	Decr(ctx context.Context, key string) *IntCmd
	DecrBy(ctx context.Context, key string, decrement int64) *IntCmd
	Get(ctx context.Context, key string) *StringCmd
	GetRange(ctx context.Context, key string, start, end int64) *StringCmd
	GetSet(ctx context.Context, key string, value interface{}) *StringCmd
	GetEx(ctx context.Context, key string, expiration time.Duration) *StringCmd
	GetDel(ctx context.Context, key string) *StringCmd
	Incr(ctx context.Context, key string) *IntCmd
	IncrBy(ctx context.Context, key string, value int64) *IntCmd
	IncrByFloat(ctx context.Context, key string, value float64) *FloatCmd
	MGet(ctx context.Context, keys ...string) *SliceCmd
	MSet(ctx context.Context, values ...interface{}) *StatusCmd
	MSetNX(ctx context.Context, values ...interface{}) *BoolCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *StatusCmd
	SetArgs(ctx context.Context, key string, value interface{}, a SetArgs) *StatusCmd
	SetEx(ctx context.Context, key string, value interface{}, expiration time.Duration) *StatusCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *BoolCmd
	SetXX(ctx context.Context, key string, value interface{}, expiration time.Duration) *BoolCmd
	SetRange(ctx context.Context, key string, offset int64, value string) *IntCmd
	StrLen(ctx context.Context, key string) *IntCmd
	Copy(ctx context.Context, sourceKey string, destKey string, db int, replace bool) *IntCmd

	GetBit(ctx context.Context, key string, offset int64) *IntCmd
	SetBit(ctx context.Context, key string, offset int64, value int) *IntCmd
	BitCount(ctx context.Context, key string, bitCount *BitCount) *IntCmd
	BitOpAnd(ctx context.Context, destKey string, keys ...string) *IntCmd
	BitOpOr(ctx context.Context, destKey string, keys ...string) *IntCmd
	BitOpXor(ctx context.Context, destKey string, keys ...string) *IntCmd
	BitOpNot(ctx context.Context, destKey string, key string) *IntCmd
	BitPos(ctx context.Context, key string, bit int64, pos ...int64) *IntCmd
	BitPosSpan(ctx context.Context, key string, bit int8, start, end int64, span string) *IntCmd
	BitField(ctx context.Context, key string, args ...interface{}) *IntSliceCmd

	Scan(ctx context.Context, cursor uint64, match string, count int64) *ScanCmd
	ScanType(ctx context.Context, cursor uint64, match string, count int64, keyType string) *ScanCmd
	SScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd
	HScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd
	ZScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd

	HDel(ctx context.Context, key string, fields ...string) *IntCmd
	HExists(ctx context.Context, key, field string) *BoolCmd
	HGet(ctx context.Context, key, field string) *StringCmd
	HGetAll(ctx context.Context, key string) *MapStringStringCmd
	HIncrBy(ctx context.Context, key, field string, incr int64) *IntCmd
	HIncrByFloat(ctx context.Context, key, field string, incr float64) *FloatCmd
	HKeys(ctx context.Context, key string) *StringSliceCmd
	HLen(ctx context.Context, key string) *IntCmd
	HMGet(ctx context.Context, key string, fields ...string) *SliceCmd
	HSet(ctx context.Context, key string, values ...interface{}) *IntCmd
	HMSet(ctx context.Context, key string, values ...interface{}) *BoolCmd
	HSetNX(ctx context.Context, key, field string, value interface{}) *BoolCmd
	HVals(ctx context.Context, key string) *StringSliceCmd
	HRandField(ctx context.Context, key string, count int) *StringSliceCmd
	HRandFieldWithValues(ctx context.Context, key string, count int) *KeyValueSliceCmd

	BLPop(ctx context.Context, timeout time.Duration, keys ...string) *StringSliceCmd
	BLMPop(ctx context.Context, timeout time.Duration, direction string, count int64, keys ...string) *KeyValuesCmd
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) *StringSliceCmd
	BRPopLPush(ctx context.Context, source, destination string, timeout time.Duration) *StringCmd
	LCS(ctx context.Context, q *LCSQuery) *LCSCmd
	LIndex(ctx context.Context, key string, index int64) *StringCmd
	LInsert(ctx context.Context, key, op string, pivot, value interface{}) *IntCmd
	LInsertBefore(ctx context.Context, key string, pivot, value interface{}) *IntCmd
	LInsertAfter(ctx context.Context, key string, pivot, value interface{}) *IntCmd
	LLen(ctx context.Context, key string) *IntCmd
	LMPop(ctx context.Context, direction string, count int64, keys ...string) *KeyValuesCmd
	LPop(ctx context.Context, key string) *StringCmd
	LPopCount(ctx context.Context, key string, count int) *StringSliceCmd
	LPos(ctx context.Context, key string, value string, args LPosArgs) *IntCmd
	LPosCount(ctx context.Context, key string, value string, count int64, args LPosArgs) *IntSliceCmd
	LPush(ctx context.Context, key string, values ...interface{}) *IntCmd
	LPushX(ctx context.Context, key string, values ...interface{}) *IntCmd
	LRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd
	LRem(ctx context.Context, key string, count int64, value interface{}) *IntCmd
	LSet(ctx context.Context, key string, index int64, value interface{}) *StatusCmd
	LTrim(ctx context.Context, key string, start, stop int64) *StatusCmd
	RPop(ctx context.Context, key string) *StringCmd
	RPopCount(ctx context.Context, key string, count int) *StringSliceCmd
	RPopLPush(ctx context.Context, source, destination string) *StringCmd
	RPush(ctx context.Context, key string, values ...interface{}) *IntCmd
	RPushX(ctx context.Context, key string, values ...interface{}) *IntCmd
	LMove(ctx context.Context, source, destination, srcpos, destpos string) *StringCmd
	BLMove(ctx context.Context, source, destination, srcpos, destpos string, timeout time.Duration) *StringCmd

	SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd
	SCard(ctx context.Context, key string) *IntCmd
	SDiff(ctx context.Context, keys ...string) *StringSliceCmd
	SDiffStore(ctx context.Context, destination string, keys ...string) *IntCmd
	SInter(ctx context.Context, keys ...string) *StringSliceCmd
	SInterCard(ctx context.Context, limit int64, keys ...string) *IntCmd
	SInterStore(ctx context.Context, destination string, keys ...string) *IntCmd
	SIsMember(ctx context.Context, key string, member interface{}) *BoolCmd
	SMIsMember(ctx context.Context, key string, members ...interface{}) *BoolSliceCmd
	SMembers(ctx context.Context, key string) *StringSliceCmd
	SMembersMap(ctx context.Context, key string) *StringStructMapCmd
	SMove(ctx context.Context, source, destination string, member interface{}) *BoolCmd
	SPop(ctx context.Context, key string) *StringCmd
	SPopN(ctx context.Context, key string, count int64) *StringSliceCmd
	SRandMember(ctx context.Context, key string) *StringCmd
	SRandMemberN(ctx context.Context, key string, count int64) *StringSliceCmd
	SRem(ctx context.Context, key string, members ...interface{}) *IntCmd
	SUnion(ctx context.Context, keys ...string) *StringSliceCmd
	SUnionStore(ctx context.Context, destination string, keys ...string) *IntCmd

	XAdd(ctx context.Context, a *XAddArgs) *StringCmd
	XDel(ctx context.Context, stream string, ids ...string) *IntCmd
	XLen(ctx context.Context, stream string) *IntCmd
	XRange(ctx context.Context, stream, start, stop string) *XMessageSliceCmd
	XRangeN(ctx context.Context, stream, start, stop string, count int64) *XMessageSliceCmd
	XRevRange(ctx context.Context, stream string, start, stop string) *XMessageSliceCmd
	XRevRangeN(ctx context.Context, stream string, start, stop string, count int64) *XMessageSliceCmd
	XRead(ctx context.Context, a *XReadArgs) *XStreamSliceCmd
	XReadStreams(ctx context.Context, streams ...string) *XStreamSliceCmd
	XGroupCreate(ctx context.Context, stream, group, start string) *StatusCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *StatusCmd
	XGroupSetID(ctx context.Context, stream, group, start string) *StatusCmd
	XGroupDestroy(ctx context.Context, stream, group string) *IntCmd
	XGroupCreateConsumer(ctx context.Context, stream, group, consumer string) *IntCmd
	XGroupDelConsumer(ctx context.Context, stream, group, consumer string) *IntCmd
	XReadGroup(ctx context.Context, a *XReadGroupArgs) *XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *IntCmd
	XPending(ctx context.Context, stream, group string) *XPendingCmd
	XPendingExt(ctx context.Context, a *XPendingExtArgs) *XPendingExtCmd
	XClaim(ctx context.Context, a *XClaimArgs) *XMessageSliceCmd
	XClaimJustID(ctx context.Context, a *XClaimArgs) *StringSliceCmd
	XAutoClaim(ctx context.Context, a *XAutoClaimArgs) *XAutoClaimCmd
	XAutoClaimJustID(ctx context.Context, a *XAutoClaimArgs) *XAutoClaimJustIDCmd
	XTrimMaxLen(ctx context.Context, key string, maxLen int64) *IntCmd
	XTrimMaxLenApprox(ctx context.Context, key string, maxLen, limit int64) *IntCmd
	XTrimMinID(ctx context.Context, key string, minID string) *IntCmd
	XTrimMinIDApprox(ctx context.Context, key string, minID string, limit int64) *IntCmd
	XInfoGroups(ctx context.Context, key string) *XInfoGroupsCmd
	XInfoStream(ctx context.Context, key string) *XInfoStreamCmd
	XInfoStreamFull(ctx context.Context, key string, count int) *XInfoStreamFullCmd
	XInfoConsumers(ctx context.Context, key string, group string) *XInfoConsumersCmd

	BZPopMax(ctx context.Context, timeout time.Duration, keys ...string) *ZWithKeyCmd
	BZPopMin(ctx context.Context, timeout time.Duration, keys ...string) *ZWithKeyCmd
	BZMPop(ctx context.Context, timeout time.Duration, order string, count int64, keys ...string) *ZSliceWithKeyCmd

	ZAdd(ctx context.Context, key string, members ...Z) *IntCmd
	ZAddLT(ctx context.Context, key string, members ...Z) *IntCmd
	ZAddGT(ctx context.Context, key string, members ...Z) *IntCmd
	ZAddNX(ctx context.Context, key string, members ...Z) *IntCmd
	ZAddXX(ctx context.Context, key string, members ...Z) *IntCmd
	ZAddArgs(ctx context.Context, key string, args ZAddArgs) *IntCmd
	ZAddArgsIncr(ctx context.Context, key string, args ZAddArgs) *FloatCmd
	ZCard(ctx context.Context, key string) *IntCmd
	ZCount(ctx context.Context, key, min, max string) *IntCmd
	ZLexCount(ctx context.Context, key, min, max string) *IntCmd
	ZIncrBy(ctx context.Context, key string, increment float64, member string) *FloatCmd
	ZInter(ctx context.Context, store *ZStore) *StringSliceCmd
	ZInterWithScores(ctx context.Context, store *ZStore) *ZSliceCmd
	ZInterCard(ctx context.Context, limit int64, keys ...string) *IntCmd
	ZInterStore(ctx context.Context, destination string, store *ZStore) *IntCmd
	ZMPop(ctx context.Context, order string, count int64, keys ...string) *ZSliceWithKeyCmd
	ZMScore(ctx context.Context, key string, members ...string) *FloatSliceCmd
	ZPopMax(ctx context.Context, key string, count ...int64) *ZSliceCmd
	ZPopMin(ctx context.Context, key string, count ...int64) *ZSliceCmd
	ZRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) *ZSliceCmd
	ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd
	ZRangeByLex(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *ZRangeBy) *ZSliceCmd
	ZRangeArgs(ctx context.Context, z ZRangeArgs) *StringSliceCmd
	ZRangeArgsWithScores(ctx context.Context, z ZRangeArgs) *ZSliceCmd
	ZRangeStore(ctx context.Context, dst string, z ZRangeArgs) *IntCmd
	ZRank(ctx context.Context, key, member string) *IntCmd
	ZRankWithScore(ctx context.Context, key, member string) *RankWithScoreCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *IntCmd
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *IntCmd
	ZRemRangeByScore(ctx context.Context, key, min, max string) *IntCmd
	ZRemRangeByLex(ctx context.Context, key, min, max string) *IntCmd
	ZRevRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *ZSliceCmd
	ZRevRangeByScore(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd
	ZRevRangeByLex(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd
	ZRevRangeByScoreWithScores(ctx context.Context, key string, opt *ZRangeBy) *ZSliceCmd
	ZRevRank(ctx context.Context, key, member string) *IntCmd
	ZRevRankWithScore(ctx context.Context, key, member string) *RankWithScoreCmd
	ZScore(ctx context.Context, key, member string) *FloatCmd
	ZUnionStore(ctx context.Context, dest string, store *ZStore) *IntCmd
	ZRandMember(ctx context.Context, key string, count int) *StringSliceCmd
	ZRandMemberWithScores(ctx context.Context, key string, count int) *ZSliceCmd
	ZUnion(ctx context.Context, store ZStore) *StringSliceCmd
	ZUnionWithScores(ctx context.Context, store ZStore) *ZSliceCmd
	ZDiff(ctx context.Context, keys ...string) *StringSliceCmd
	ZDiffWithScores(ctx context.Context, keys ...string) *ZSliceCmd
	ZDiffStore(ctx context.Context, destination string, keys ...string) *IntCmd

	PFAdd(ctx context.Context, key string, els ...interface{}) *IntCmd
	PFCount(ctx context.Context, keys ...string) *IntCmd
	PFMerge(ctx context.Context, dest string, keys ...string) *StatusCmd

	BgRewriteAOF(ctx context.Context) *StatusCmd
	BgSave(ctx context.Context) *StatusCmd
	ClientKill(ctx context.Context, ipPort string) *StatusCmd
	ClientKillByFilter(ctx context.Context, keys ...string) *IntCmd
	ClientList(ctx context.Context) *StringCmd
	ClientInfo(ctx context.Context) *ClientInfoCmd
	ClientPause(ctx context.Context, dur time.Duration) *BoolCmd
	ClientUnpause(ctx context.Context) *BoolCmd
	ClientID(ctx context.Context) *IntCmd
	ClientUnblock(ctx context.Context, id int64) *IntCmd
	ClientUnblockWithError(ctx context.Context, id int64) *IntCmd
	ConfigGet(ctx context.Context, parameter string) *MapStringStringCmd
	ConfigResetStat(ctx context.Context) *StatusCmd
	ConfigSet(ctx context.Context, parameter, value string) *StatusCmd
	ConfigRewrite(ctx context.Context) *StatusCmd
	DBSize(ctx context.Context) *IntCmd
	FlushAll(ctx context.Context) *StatusCmd
	FlushAllAsync(ctx context.Context) *StatusCmd
	FlushDB(ctx context.Context) *StatusCmd
	FlushDBAsync(ctx context.Context) *StatusCmd
	Info(ctx context.Context, section ...string) *StringCmd
	LastSave(ctx context.Context) *IntCmd
	Save(ctx context.Context) *StatusCmd
	Shutdown(ctx context.Context) *StatusCmd
	ShutdownSave(ctx context.Context) *StatusCmd
	ShutdownNoSave(ctx context.Context) *StatusCmd
	SlaveOf(ctx context.Context, host, port string) *StatusCmd
	SlowLogGet(ctx context.Context, num int64) *SlowLogCmd
	Time(ctx context.Context) *TimeCmd
	DebugObject(ctx context.Context, key string) *StringCmd
	ReadOnly(ctx context.Context) *StatusCmd
	ReadWrite(ctx context.Context) *StatusCmd
	MemoryUsage(ctx context.Context, key string, samples ...int) *IntCmd

	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *Cmd
	EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) *Cmd
	EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) *Cmd
	ScriptExists(ctx context.Context, hashes ...string) *BoolSliceCmd
	ScriptFlush(ctx context.Context) *StatusCmd
	ScriptKill(ctx context.Context) *StatusCmd
	ScriptLoad(ctx context.Context, script string) *StringCmd

	FunctionLoad(ctx context.Context, code string) *StringCmd
	FunctionLoadReplace(ctx context.Context, code string) *StringCmd
	FunctionDelete(ctx context.Context, libName string) *StringCmd
	FunctionFlush(ctx context.Context) *StringCmd
	FunctionKill(ctx context.Context) *StringCmd
	FunctionFlushAsync(ctx context.Context) *StringCmd
	FunctionList(ctx context.Context, q FunctionListQuery) *FunctionListCmd
	FunctionDump(ctx context.Context) *StringCmd
	FunctionRestore(ctx context.Context, libDump string) *StringCmd
	FunctionStats(ctx context.Context) *FunctionStatsCmd
	FCall(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd
	FCallRo(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd
	FCallRO(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd

	Publish(ctx context.Context, channel string, message interface{}) *IntCmd
	SPublish(ctx context.Context, channel string, message interface{}) *IntCmd
	PubSubChannels(ctx context.Context, pattern string) *StringSliceCmd
	PubSubNumSub(ctx context.Context, channels ...string) *MapStringIntCmd
	PubSubNumPat(ctx context.Context) *IntCmd
	PubSubShardChannels(ctx context.Context, pattern string) *StringSliceCmd
	PubSubShardNumSub(ctx context.Context, channels ...string) *MapStringIntCmd

	ClusterMyShardID(ctx context.Context) *StringCmd
	ClusterSlots(ctx context.Context) *ClusterSlotsCmd
	ClusterShards(ctx context.Context) *ClusterShardsCmd
	ClusterLinks(ctx context.Context) *ClusterLinksCmd
	ClusterNodes(ctx context.Context) *StringCmd
	ClusterMeet(ctx context.Context, host, port string) *StatusCmd
	ClusterForget(ctx context.Context, nodeID string) *StatusCmd
	ClusterReplicate(ctx context.Context, nodeID string) *StatusCmd
	ClusterResetSoft(ctx context.Context) *StatusCmd
	ClusterResetHard(ctx context.Context) *StatusCmd
	ClusterInfo(ctx context.Context) *StringCmd
	ClusterKeySlot(ctx context.Context, key string) *IntCmd
	ClusterGetKeysInSlot(ctx context.Context, slot int, count int) *StringSliceCmd
	ClusterCountFailureReports(ctx context.Context, nodeID string) *IntCmd
	ClusterCountKeysInSlot(ctx context.Context, slot int) *IntCmd
	ClusterDelSlots(ctx context.Context, slots ...int) *StatusCmd
	ClusterDelSlotsRange(ctx context.Context, min, max int) *StatusCmd
	ClusterSaveConfig(ctx context.Context) *StatusCmd
	ClusterSlaves(ctx context.Context, nodeID string) *StringSliceCmd
	ClusterFailover(ctx context.Context) *StatusCmd
	ClusterAddSlots(ctx context.Context, slots ...int) *StatusCmd
	ClusterAddSlotsRange(ctx context.Context, min, max int) *StatusCmd

	GeoAdd(ctx context.Context, key string, geoLocation ...*GeoLocation) *IntCmd
	GeoPos(ctx context.Context, key string, members ...string) *GeoPosCmd
	GeoRadius(ctx context.Context, key string, longitude, latitude float64, query *GeoRadiusQuery) *GeoLocationCmd
	GeoRadiusStore(ctx context.Context, key string, longitude, latitude float64, query *GeoRadiusQuery) *IntCmd
	GeoRadiusByMember(ctx context.Context, key, member string, query *GeoRadiusQuery) *GeoLocationCmd
	GeoRadiusByMemberStore(ctx context.Context, key, member string, query *GeoRadiusQuery) *IntCmd
	GeoSearch(ctx context.Context, key string, q *GeoSearchQuery) *StringSliceCmd
	GeoSearchLocation(ctx context.Context, key string, q *GeoSearchLocationQuery) *GeoSearchLocationCmd
	GeoSearchStore(ctx context.Context, key, store string, q *GeoSearchStoreQuery) *IntCmd
	GeoDist(ctx context.Context, key string, member1, member2, unit string) *FloatCmd
	GeoHash(ctx context.Context, key string, members ...string) *StringSliceCmd

	ACLDryRun(ctx context.Context, username string, command ...interface{}) *StringCmd
	ACLLog(ctx context.Context, count int64) *ACLLogCmd
	ACLLogReset(ctx context.Context) *StatusCmd

	ModuleLoadex(ctx context.Context, conf *ModuleLoadexConfig) *StringCmd
}

type StatefulCmdable interface {
	Cmdable
	Auth(ctx context.Context, password string) *StatusCmd
	AuthACL(ctx context.Context, username, password string) *StatusCmd
	Select(ctx context.Context, index int) *StatusCmd
	SwapDB(ctx context.Context, index1, index2 int) *StatusCmd
	ClientSetName(ctx context.Context, name string) *BoolCmd
	Hello(ctx context.Context, ver int, username, password, clientName string) *MapStringInterfaceCmd
}

var (
	_ Cmdable = (*Client)(nil)
	_ Cmdable = (*Tx)(nil)
	_ Cmdable = (*Ring)(nil)
	_ Cmdable = (*ClusterClient)(nil)
)

type cmdable func(ctx context.Context, cmd Cmder) error

type statefulCmdable func(ctx context.Context, cmd Cmder) error

//------------------------------------------------------------------------------

func (c statefulCmdable) Auth(ctx context.Context, password string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "auth", password)
	_ = c(ctx, cmd)
	return cmd
}

// AuthACL Perform an AUTH command, using the given user and pass.
// Should be used to authenticate the current connection with one of the connections defined in the ACL list
// when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
func (c statefulCmdable) AuthACL(ctx context.Context, username, password string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "auth", username, password)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Wait(ctx context.Context, numSlaves int, timeout time.Duration) *IntCmd {
	cmd := NewIntCmd(ctx, "wait", numSlaves, int(timeout/time.Millisecond))
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c statefulCmdable) Select(ctx context.Context, index int) *StatusCmd {
	cmd := NewStatusCmd(ctx, "select", index)
	_ = c(ctx, cmd)
	return cmd
}

func (c statefulCmdable) SwapDB(ctx context.Context, index1, index2 int) *StatusCmd {
	cmd := NewStatusCmd(ctx, "swapdb", index1, index2)
	_ = c(ctx, cmd)
	return cmd
}

// ClientSetName assigns a name to the connection.
func (c statefulCmdable) ClientSetName(ctx context.Context, name string) *BoolCmd {
	cmd := NewBoolCmd(ctx, "client", "setname", name)
	_ = c(ctx, cmd)
	return cmd
}

// Hello Set the resp protocol used.
func (c statefulCmdable) Hello(ctx context.Context,
	ver int, username, password, clientName string) *MapStringInterfaceCmd {
	args := make([]interface{}, 0, 7)
	args = append(args, "hello", ver)
	if password != "" {
		if username != "" {
			args = append(args, "auth", username, password)
		} else {
			args = append(args, "auth", "default", password)
		}
	}
	if clientName != "" {
		args = append(args, "setname", clientName)
	}
	cmd := NewMapStringInterfaceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Command(ctx context.Context) *CommandsInfoCmd {
	cmd := NewCommandsInfoCmd(ctx, "command")
	_ = c(ctx, cmd)
	return cmd
}

// FilterBy is used for the `CommandList` command parameter.
type FilterBy struct {
	Module  string
	ACLCat  string
	Pattern string
}

func (c cmdable) CommandList(ctx context.Context, filter *FilterBy) *StringSliceCmd {
	args := make([]interface{}, 0, 5)
	args = append(args, "command", "list")
	if filter != nil {
		if filter.Module != "" {
			args = append(args, "filterby", "module", filter.Module)
		} else if filter.ACLCat != "" {
			args = append(args, "filterby", "aclcat", filter.ACLCat)
		} else if filter.Pattern != "" {
			args = append(args, "filterby", "pattern", filter.Pattern)
		}
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) CommandGetKeys(ctx context.Context, commands ...interface{}) *StringSliceCmd {
	args := make([]interface{}, 2+len(commands))
	args[0] = "command"
	args[1] = "getkeys"
	copy(args[2:], commands)
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) CommandGetKeysAndFlags(ctx context.Context, commands ...interface{}) *KeyFlagsCmd {
	args := make([]interface{}, 2+len(commands))
	args[0] = "command"
	args[1] = "getkeysandflags"
	copy(args[2:], commands)
	cmd := NewKeyFlagsCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// ClientGetName returns the name of the connection.
func (c cmdable) ClientGetName(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "client", "getname")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Echo(ctx context.Context, message interface{}) *StringCmd {
	cmd := NewStringCmd(ctx, "echo", message)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Ping(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "ping")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Quit(_ context.Context) *StatusCmd {
	panic("not implemented")
}

func (c cmdable) Del(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "del"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Unlink(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "unlink"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Dump(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "dump", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Exists(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "exists"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Expire(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	return c.expire(ctx, key, expiration, "")
}

func (c cmdable) ExpireNX(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	return c.expire(ctx, key, expiration, "NX")
}

func (c cmdable) ExpireXX(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	return c.expire(ctx, key, expiration, "XX")
}

func (c cmdable) ExpireGT(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	return c.expire(ctx, key, expiration, "GT")
}

func (c cmdable) ExpireLT(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	return c.expire(ctx, key, expiration, "LT")
}

func (c cmdable) expire(
	ctx context.Context, key string, expiration time.Duration, mode string,
) *BoolCmd {
	args := make([]interface{}, 3, 4)
	args[0] = "expire"
	args[1] = key
	args[2] = formatSec(ctx, expiration)
	if mode != "" {
		args = append(args, mode)
	}

	cmd := NewBoolCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ExpireAt(ctx context.Context, key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd(ctx, "expireat", key, tm.Unix())
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ExpireTime(ctx context.Context, key string) *DurationCmd {
	cmd := NewDurationCmd(ctx, time.Second, "expiretime", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Keys(ctx context.Context, pattern string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "keys", pattern)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Migrate(ctx context.Context, host, port, key string, db int, timeout time.Duration) *StatusCmd {
	cmd := NewStatusCmd(
		ctx,
		"migrate",
		host,
		port,
		key,
		db,
		formatMs(ctx, timeout),
	)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Move(ctx context.Context, key string, db int) *BoolCmd {
	cmd := NewBoolCmd(ctx, "move", key, db)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ObjectRefCount(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "object", "refcount", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ObjectEncoding(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "object", "encoding", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ObjectIdleTime(ctx context.Context, key string) *DurationCmd {
	cmd := NewDurationCmd(ctx, time.Second, "object", "idletime", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Persist(ctx context.Context, key string) *BoolCmd {
	cmd := NewBoolCmd(ctx, "persist", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PExpire(ctx context.Context, key string, expiration time.Duration) *BoolCmd {
	cmd := NewBoolCmd(ctx, "pexpire", key, formatMs(ctx, expiration))
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PExpireAt(ctx context.Context, key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd(
		ctx,
		"pexpireat",
		key,
		tm.UnixNano()/int64(time.Millisecond),
	)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PExpireTime(ctx context.Context, key string) *DurationCmd {
	cmd := NewDurationCmd(ctx, time.Millisecond, "pexpiretime", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PTTL(ctx context.Context, key string) *DurationCmd {
	cmd := NewDurationCmd(ctx, time.Millisecond, "pttl", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RandomKey(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "randomkey")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Rename(ctx context.Context, key, newkey string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "rename", key, newkey)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RenameNX(ctx context.Context, key, newkey string) *BoolCmd {
	cmd := NewBoolCmd(ctx, "renamenx", key, newkey)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Restore(ctx context.Context, key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		ctx,
		"restore",
		key,
		formatMs(ctx, ttl),
		value,
	)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RestoreReplace(ctx context.Context, key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		ctx,
		"restore",
		key,
		formatMs(ctx, ttl),
		value,
		"replace",
	)
	_ = c(ctx, cmd)
	return cmd
}

type Sort struct {
	By            string
	Offset, Count int64
	Get           []string
	Order         string
	Alpha         bool
}

func (sort *Sort) args(command, key string) []interface{} {
	args := []interface{}{command, key}

	if sort.By != "" {
		args = append(args, "by", sort.By)
	}
	if sort.Offset != 0 || sort.Count != 0 {
		args = append(args, "limit", sort.Offset, sort.Count)
	}
	for _, get := range sort.Get {
		args = append(args, "get", get)
	}
	if sort.Order != "" {
		args = append(args, sort.Order)
	}
	if sort.Alpha {
		args = append(args, "alpha")
	}
	return args
}

func (c cmdable) SortRO(ctx context.Context, key string, sort *Sort) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, sort.args("sort_ro", key)...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Sort(ctx context.Context, key string, sort *Sort) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, sort.args("sort", key)...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SortStore(ctx context.Context, key, store string, sort *Sort) *IntCmd {
	args := sort.args("sort", key)
	if store != "" {
		args = append(args, "store", store)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SortInterfaces(ctx context.Context, key string, sort *Sort) *SliceCmd {
	cmd := NewSliceCmd(ctx, sort.args("sort", key)...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Touch(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, len(keys)+1)
	args[0] = "touch"
	for i, key := range keys {
		args[i+1] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) TTL(ctx context.Context, key string) *DurationCmd {
	cmd := NewDurationCmd(ctx, time.Second, "ttl", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Type(ctx context.Context, key string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "type", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Append(ctx context.Context, key, value string) *IntCmd {
	cmd := NewIntCmd(ctx, "append", key, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Decr(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "decr", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) DecrBy(ctx context.Context, key string, decrement int64) *IntCmd {
	cmd := NewIntCmd(ctx, "decrby", key, decrement)
	_ = c(ctx, cmd)
	return cmd
}

// Get Redis `GET key` command. It returns redis.Nil error when key does not exist.
func (c cmdable) Get(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "get", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GetRange(ctx context.Context, key string, start, end int64) *StringCmd {
	cmd := NewStringCmd(ctx, "getrange", key, start, end)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GetSet(ctx context.Context, key string, value interface{}) *StringCmd {
	cmd := NewStringCmd(ctx, "getset", key, value)
	_ = c(ctx, cmd)
	return cmd
}

// GetEx An expiration of zero removes the TTL associated with the key (i.e. GETEX key persist).
// Requires Redis >= 6.2.0.
func (c cmdable) GetEx(ctx context.Context, key string, expiration time.Duration) *StringCmd {
	args := make([]interface{}, 0, 4)
	args = append(args, "getex", key)
	if expiration > 0 {
		if usePrecise(expiration) {
			args = append(args, "px", formatMs(ctx, expiration))
		} else {
			args = append(args, "ex", formatSec(ctx, expiration))
		}
	} else if expiration == 0 {
		args = append(args, "persist")
	}

	cmd := NewStringCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// GetDel redis-server version >= 6.2.0.
func (c cmdable) GetDel(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "getdel", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Incr(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "incr", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) IncrBy(ctx context.Context, key string, value int64) *IntCmd {
	cmd := NewIntCmd(ctx, "incrby", key, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) IncrByFloat(ctx context.Context, key string, value float64) *FloatCmd {
	cmd := NewFloatCmd(ctx, "incrbyfloat", key, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) MGet(ctx context.Context, keys ...string) *SliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "mget"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// MSet is like Set but accepts multiple values:
//   - MSet("key1", "value1", "key2", "value2")
//   - MSet([]string{"key1", "value1", "key2", "value2"})
//   - MSet(map[string]interface{}{"key1": "value1", "key2": "value2"})
//   - MSet(struct), For struct types, see HSet description.
func (c cmdable) MSet(ctx context.Context, values ...interface{}) *StatusCmd {
	args := make([]interface{}, 1, 1+len(values))
	args[0] = "mset"
	args = appendArgs(args, values)
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// MSetNX is like SetNX but accepts multiple values:
//   - MSetNX("key1", "value1", "key2", "value2")
//   - MSetNX([]string{"key1", "value1", "key2", "value2"})
//   - MSetNX(map[string]interface{}{"key1": "value1", "key2": "value2"})
//   - MSetNX(struct), For struct types, see HSet description.
func (c cmdable) MSetNX(ctx context.Context, values ...interface{}) *BoolCmd {
	args := make([]interface{}, 1, 1+len(values))
	args[0] = "msetnx"
	args = appendArgs(args, values)
	cmd := NewBoolCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// Set Redis `SET key value [expiration]` command.
// Use expiration for `SETEx`-like behavior.
//
// Zero expiration means the key has no expiration time.
// KeepTTL is a Redis KEEPTTL option to keep existing TTL, it requires your redis-server version >= 6.0,
// otherwise you will receive an error: (error) ERR syntax error.
func (c cmdable) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *StatusCmd {
	args := make([]interface{}, 3, 5)
	args[0] = "set"
	args[1] = key
	args[2] = value
	if expiration > 0 {
		if usePrecise(expiration) {
			args = append(args, "px", formatMs(ctx, expiration))
		} else {
			args = append(args, "ex", formatSec(ctx, expiration))
		}
	} else if expiration == KeepTTL {
		args = append(args, "keepttl")
	}

	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// SetArgs provides arguments for the SetArgs function.
type SetArgs struct {
	// Mode can be `NX` or `XX` or empty.
	Mode string

	// Zero `TTL` or `Expiration` means that the key has no expiration time.
	TTL      time.Duration
	ExpireAt time.Time

	// When Get is true, the command returns the old value stored at key, or nil when key did not exist.
	Get bool

	// KeepTTL is a Redis KEEPTTL option to keep existing TTL, it requires your redis-server version >= 6.0,
	// otherwise you will receive an error: (error) ERR syntax error.
	KeepTTL bool
}

// SetArgs supports all the options that the SET command supports.
// It is the alternative to the Set function when you want
// to have more control over the options.
func (c cmdable) SetArgs(ctx context.Context, key string, value interface{}, a SetArgs) *StatusCmd {
	args := []interface{}{"set", key, value}

	if a.KeepTTL {
		args = append(args, "keepttl")
	}

	if !a.ExpireAt.IsZero() {
		args = append(args, "exat", a.ExpireAt.Unix())
	}
	if a.TTL > 0 {
		if usePrecise(a.TTL) {
			args = append(args, "px", formatMs(ctx, a.TTL))
		} else {
			args = append(args, "ex", formatSec(ctx, a.TTL))
		}
	}

	if a.Mode != "" {
		args = append(args, a.Mode)
	}

	if a.Get {
		args = append(args, "get")
	}

	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// SetEx Redis `SETEx key expiration value` command.
func (c cmdable) SetEx(ctx context.Context, key string, value interface{}, expiration time.Duration) *StatusCmd {
	cmd := NewStatusCmd(ctx, "setex", key, formatSec(ctx, expiration), value)
	_ = c(ctx, cmd)
	return cmd
}

// SetNX Redis `SET key value [expiration] NX` command.
//
// Zero expiration means the key has no expiration time.
// KeepTTL is a Redis KEEPTTL option to keep existing TTL, it requires your redis-server version >= 6.0,
// otherwise you will receive an error: (error) ERR syntax error.
func (c cmdable) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	switch expiration {
	case 0:
		// Use old `SETNX` to support old Redis versions.
		cmd = NewBoolCmd(ctx, "setnx", key, value)
	case KeepTTL:
		cmd = NewBoolCmd(ctx, "set", key, value, "keepttl", "nx")
	default:
		if usePrecise(expiration) {
			cmd = NewBoolCmd(ctx, "set", key, value, "px", formatMs(ctx, expiration), "nx")
		} else {
			cmd = NewBoolCmd(ctx, "set", key, value, "ex", formatSec(ctx, expiration), "nx")
		}
	}

	_ = c(ctx, cmd)
	return cmd
}

// SetXX Redis `SET key value [expiration] XX` command.
//
// Zero expiration means the key has no expiration time.
// KeepTTL is a Redis KEEPTTL option to keep existing TTL, it requires your redis-server version >= 6.0,
// otherwise you will receive an error: (error) ERR syntax error.
func (c cmdable) SetXX(ctx context.Context, key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	switch expiration {
	case 0:
		cmd = NewBoolCmd(ctx, "set", key, value, "xx")
	case KeepTTL:
		cmd = NewBoolCmd(ctx, "set", key, value, "keepttl", "xx")
	default:
		if usePrecise(expiration) {
			cmd = NewBoolCmd(ctx, "set", key, value, "px", formatMs(ctx, expiration), "xx")
		} else {
			cmd = NewBoolCmd(ctx, "set", key, value, "ex", formatSec(ctx, expiration), "xx")
		}
	}

	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SetRange(ctx context.Context, key string, offset int64, value string) *IntCmd {
	cmd := NewIntCmd(ctx, "setrange", key, offset, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) StrLen(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "strlen", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Copy(ctx context.Context, sourceKey, destKey string, db int, replace bool) *IntCmd {
	args := []interface{}{"copy", sourceKey, destKey, "DB", db}
	if replace {
		args = append(args, "REPLACE")
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) GetBit(ctx context.Context, key string, offset int64) *IntCmd {
	cmd := NewIntCmd(ctx, "getbit", key, offset)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SetBit(ctx context.Context, key string, offset int64, value int) *IntCmd {
	cmd := NewIntCmd(
		ctx,
		"setbit",
		key,
		offset,
		value,
	)
	_ = c(ctx, cmd)
	return cmd
}

type BitCount struct {
	Start, End int64
}

func (c cmdable) BitCount(ctx context.Context, key string, bitCount *BitCount) *IntCmd {
	args := []interface{}{"bitcount", key}
	if bitCount != nil {
		args = append(
			args,
			bitCount.Start,
			bitCount.End,
		)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) bitOp(ctx context.Context, op, destKey string, keys ...string) *IntCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "bitop"
	args[1] = op
	args[2] = destKey
	for i, key := range keys {
		args[3+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BitOpAnd(ctx context.Context, destKey string, keys ...string) *IntCmd {
	return c.bitOp(ctx, "and", destKey, keys...)
}

func (c cmdable) BitOpOr(ctx context.Context, destKey string, keys ...string) *IntCmd {
	return c.bitOp(ctx, "or", destKey, keys...)
}

func (c cmdable) BitOpXor(ctx context.Context, destKey string, keys ...string) *IntCmd {
	return c.bitOp(ctx, "xor", destKey, keys...)
}

func (c cmdable) BitOpNot(ctx context.Context, destKey string, key string) *IntCmd {
	return c.bitOp(ctx, "not", destKey, key)
}

// BitPos is an API before Redis version 7.0, cmd: bitpos key bit start end
// if you need the `byte | bit` parameter, please use `BitPosSpan`.
func (c cmdable) BitPos(ctx context.Context, key string, bit int64, pos ...int64) *IntCmd {
	args := make([]interface{}, 3+len(pos))
	args[0] = "bitpos"
	args[1] = key
	args[2] = bit
	switch len(pos) {
	case 0:
	case 1:
		args[3] = pos[0]
	case 2:
		args[3] = pos[0]
		args[4] = pos[1]
	default:
		panic("too many arguments")
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// BitPosSpan supports the `byte | bit` parameters in redis version 7.0,
// the bitpos command defaults to using byte type for the `start-end` range,
// which means it counts in bytes from start to end. you can set the value
// of "span" to determine the type of `start-end`.
// span = "bit", cmd: bitpos key bit start end bit
// span = "byte", cmd: bitpos key bit start end byte
func (c cmdable) BitPosSpan(ctx context.Context, key string, bit int8, start, end int64, span string) *IntCmd {
	cmd := NewIntCmd(ctx, "bitpos", key, bit, start, end, span)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BitField(ctx context.Context, key string, args ...interface{}) *IntSliceCmd {
	a := make([]interface{}, 0, 2+len(args))
	a = append(a, "bitfield")
	a = append(a, key)
	a = append(a, args...)
	cmd := NewIntSliceCmd(ctx, a...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Scan(ctx context.Context, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"scan", cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ScanType(ctx context.Context, cursor uint64, match string, count int64, keyType string) *ScanCmd {
	args := []interface{}{"scan", cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	if keyType != "" {
		args = append(args, "type", keyType)
	}
	cmd := NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"sscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"hscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZScan(ctx context.Context, key string, cursor uint64, match string, count int64) *ScanCmd {
	args := []interface{}{"zscan", key, cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewScanCmd(ctx, c, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) HDel(ctx context.Context, key string, fields ...string) *IntCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "hdel"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HExists(ctx context.Context, key, field string) *BoolCmd {
	cmd := NewBoolCmd(ctx, "hexists", key, field)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HGet(ctx context.Context, key, field string) *StringCmd {
	cmd := NewStringCmd(ctx, "hget", key, field)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HGetAll(ctx context.Context, key string) *MapStringStringCmd {
	cmd := NewMapStringStringCmd(ctx, "hgetall", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HIncrBy(ctx context.Context, key, field string, incr int64) *IntCmd {
	cmd := NewIntCmd(ctx, "hincrby", key, field, incr)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HIncrByFloat(ctx context.Context, key, field string, incr float64) *FloatCmd {
	cmd := NewFloatCmd(ctx, "hincrbyfloat", key, field, incr)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HKeys(ctx context.Context, key string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "hkeys", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HLen(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "hlen", key)
	_ = c(ctx, cmd)
	return cmd
}

// HMGet returns the values for the specified fields in the hash stored at key.
// It returns an interface{} to distinguish between empty string and nil value.
func (c cmdable) HMGet(ctx context.Context, key string, fields ...string) *SliceCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "hmget"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// HSet accepts values in following formats:
//
//   - HSet("myhash", "key1", "value1", "key2", "value2")
//
//   - HSet("myhash", []string{"key1", "value1", "key2", "value2"})
//
//   - HSet("myhash", map[string]interface{}{"key1": "value1", "key2": "value2"})
//
//     Playing struct With "redis" tag.
//     type MyHash struct { Key1 string `redis:"key1"`; Key2 int `redis:"key2"` }
//
//   - HSet("myhash", MyHash{"value1", "value2"}) Warn: redis-server >= 4.0
//
//     For struct, can be a structure pointer type, we only parse the field whose tag is redis.
//     if you don't want the field to be read, you can use the `redis:"-"` flag to ignore it,
//     or you don't need to set the redis tag.
//     For the type of structure field, we only support simple data types:
//     string, int/uint(8,16,32,64), float(32,64), time.Time(to RFC3339Nano), time.Duration(to Nanoseconds ),
//     if you are other more complex or custom data types, please implement the encoding.BinaryMarshaler interface.
//
// Note that in older versions of Redis server(redis-server < 4.0), HSet only supports a single key-value pair.
// redis-docs: https://redis.io/commands/hset (Starting with Redis version 4.0.0: Accepts multiple field and value arguments.)
// If you are using a Struct type and the number of fields is greater than one,
// you will receive an error similar to "ERR wrong number of arguments", you can use HMSet as a substitute.
func (c cmdable) HSet(ctx context.Context, key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "hset"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// HMSet is a deprecated version of HSet left for compatibility with Redis 3.
func (c cmdable) HMSet(ctx context.Context, key string, values ...interface{}) *BoolCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "hmset"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewBoolCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HSetNX(ctx context.Context, key, field string, value interface{}) *BoolCmd {
	cmd := NewBoolCmd(ctx, "hsetnx", key, field, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) HVals(ctx context.Context, key string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "hvals", key)
	_ = c(ctx, cmd)
	return cmd
}

// HRandField redis-server version >= 6.2.0.
func (c cmdable) HRandField(ctx context.Context, key string, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "hrandfield", key, count)
	_ = c(ctx, cmd)
	return cmd
}

// HRandFieldWithValues redis-server version >= 6.2.0.
func (c cmdable) HRandFieldWithValues(ctx context.Context, key string, count int) *KeyValueSliceCmd {
	cmd := NewKeyValueSliceCmd(ctx, "hrandfield", key, count, "withvalues")
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) BLPop(ctx context.Context, timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "blpop"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(ctx, timeout)
	cmd := NewStringSliceCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BLMPop(ctx context.Context, timeout time.Duration, direction string, count int64, keys ...string) *KeyValuesCmd {
	args := make([]interface{}, 3+len(keys), 6+len(keys))
	args[0] = "blmpop"
	args[1] = formatSec(ctx, timeout)
	args[2] = len(keys)
	for i, key := range keys {
		args[3+i] = key
	}
	args = append(args, strings.ToLower(direction), "count", count)
	cmd := NewKeyValuesCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BRPop(ctx context.Context, timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "brpop"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(keys)+1] = formatSec(ctx, timeout)
	cmd := NewStringSliceCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BRPopLPush(ctx context.Context, source, destination string, timeout time.Duration) *StringCmd {
	cmd := NewStringCmd(
		ctx,
		"brpoplpush",
		source,
		destination,
		formatSec(ctx, timeout),
	)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LCS(ctx context.Context, q *LCSQuery) *LCSCmd {
	cmd := NewLCSCmd(ctx, q)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LIndex(ctx context.Context, key string, index int64) *StringCmd {
	cmd := NewStringCmd(ctx, "lindex", key, index)
	_ = c(ctx, cmd)
	return cmd
}

// LMPop Pops one or more elements from the first non-empty list key from the list of provided key names.
// direction: left or right, count: > 0
// example: client.LMPop(ctx, "left", 3, "key1", "key2")
func (c cmdable) LMPop(ctx context.Context, direction string, count int64, keys ...string) *KeyValuesCmd {
	args := make([]interface{}, 2+len(keys), 5+len(keys))
	args[0] = "lmpop"
	args[1] = len(keys)
	for i, key := range keys {
		args[2+i] = key
	}
	args = append(args, strings.ToLower(direction), "count", count)
	cmd := NewKeyValuesCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LInsert(ctx context.Context, key, op string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "linsert", key, op, pivot, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LInsertBefore(ctx context.Context, key string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "linsert", key, "before", pivot, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LInsertAfter(ctx context.Context, key string, pivot, value interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "linsert", key, "after", pivot, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LLen(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "llen", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LPop(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "lpop", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LPopCount(ctx context.Context, key string, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "lpop", key, count)
	_ = c(ctx, cmd)
	return cmd
}

type LPosArgs struct {
	Rank, MaxLen int64
}

func (c cmdable) LPos(ctx context.Context, key string, value string, a LPosArgs) *IntCmd {
	args := []interface{}{"lpos", key, value}
	if a.Rank != 0 {
		args = append(args, "rank", a.Rank)
	}
	if a.MaxLen != 0 {
		args = append(args, "maxlen", a.MaxLen)
	}

	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LPosCount(ctx context.Context, key string, value string, count int64, a LPosArgs) *IntSliceCmd {
	args := []interface{}{"lpos", key, value, "count", count}
	if a.Rank != 0 {
		args = append(args, "rank", a.Rank)
	}
	if a.MaxLen != 0 {
		args = append(args, "maxlen", a.MaxLen)
	}
	cmd := NewIntSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LPush(ctx context.Context, key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "lpush"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LPushX(ctx context.Context, key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "lpushx"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(
		ctx,
		"lrange",
		key,
		start,
		stop,
	)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LRem(ctx context.Context, key string, count int64, value interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "lrem", key, count, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LSet(ctx context.Context, key string, index int64, value interface{}) *StatusCmd {
	cmd := NewStatusCmd(ctx, "lset", key, index, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LTrim(ctx context.Context, key string, start, stop int64) *StatusCmd {
	cmd := NewStatusCmd(
		ctx,
		"ltrim",
		key,
		start,
		stop,
	)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RPop(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "rpop", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RPopCount(ctx context.Context, key string, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "rpop", key, count)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RPopLPush(ctx context.Context, source, destination string) *StringCmd {
	cmd := NewStringCmd(ctx, "rpoplpush", source, destination)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RPush(ctx context.Context, key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "rpush"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) RPushX(ctx context.Context, key string, values ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(values))
	args[0] = "rpushx"
	args[1] = key
	args = appendArgs(args, values)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LMove(ctx context.Context, source, destination, srcpos, destpos string) *StringCmd {
	cmd := NewStringCmd(ctx, "lmove", source, destination, srcpos, destpos)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BLMove(
	ctx context.Context, source, destination, srcpos, destpos string, timeout time.Duration,
) *StringCmd {
	cmd := NewStringCmd(ctx, "blmove", source, destination, srcpos, destpos, formatSec(ctx, timeout))
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "sadd"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SCard(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "scard", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SDiff(ctx context.Context, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sdiff"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SDiffStore(ctx context.Context, destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sdiffstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SInter(ctx context.Context, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sinter"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SInterCard(ctx context.Context, limit int64, keys ...string) *IntCmd {
	args := make([]interface{}, 4+len(keys))
	args[0] = "sintercard"
	numkeys := int64(0)
	for i, key := range keys {
		args[2+i] = key
		numkeys++
	}
	args[1] = numkeys
	args[2+numkeys] = "limit"
	args[3+numkeys] = limit
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SInterStore(ctx context.Context, destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sinterstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SIsMember(ctx context.Context, key string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd(ctx, "sismember", key, member)
	_ = c(ctx, cmd)
	return cmd
}

// SMIsMember Redis `SMISMEMBER key member [member ...]` command.
func (c cmdable) SMIsMember(ctx context.Context, key string, members ...interface{}) *BoolSliceCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "smismember"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewBoolSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// SMembers Redis `SMEMBERS key` command output as a slice.
func (c cmdable) SMembers(ctx context.Context, key string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "smembers", key)
	_ = c(ctx, cmd)
	return cmd
}

// SMembersMap Redis `SMEMBERS key` command output as a map.
func (c cmdable) SMembersMap(ctx context.Context, key string) *StringStructMapCmd {
	cmd := NewStringStructMapCmd(ctx, "smembers", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SMove(ctx context.Context, source, destination string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd(ctx, "smove", source, destination, member)
	_ = c(ctx, cmd)
	return cmd
}

// SPop Redis `SPOP key` command.
func (c cmdable) SPop(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "spop", key)
	_ = c(ctx, cmd)
	return cmd
}

// SPopN Redis `SPOP key count` command.
func (c cmdable) SPopN(ctx context.Context, key string, count int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "spop", key, count)
	_ = c(ctx, cmd)
	return cmd
}

// SRandMember Redis `SRANDMEMBER key` command.
func (c cmdable) SRandMember(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "srandmember", key)
	_ = c(ctx, cmd)
	return cmd
}

// SRandMemberN Redis `SRANDMEMBER key count` command.
func (c cmdable) SRandMemberN(ctx context.Context, key string, count int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "srandmember", key, count)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "srem"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SUnion(ctx context.Context, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "sunion"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SUnionStore(ctx context.Context, destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "sunionstore"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

// XAddArgs accepts values in the following formats:
//   - XAddArgs.Values = []interface{}{"key1", "value1", "key2", "value2"}
//   - XAddArgs.Values = []string("key1", "value1", "key2", "value2")
//   - XAddArgs.Values = map[string]interface{}{"key1": "value1", "key2": "value2"}
//
// Note that map will not preserve the order of key-value pairs.
// MaxLen/MaxLenApprox and MinID are in conflict, only one of them can be used.
type XAddArgs struct {
	Stream     string
	NoMkStream bool
	MaxLen     int64 // MAXLEN N
	MinID      string
	// Approx causes MaxLen and MinID to use "~" matcher (instead of "=").
	Approx bool
	Limit  int64
	ID     string
	Values interface{}
}

func (c cmdable) XAdd(ctx context.Context, a *XAddArgs) *StringCmd {
	args := make([]interface{}, 0, 11)
	args = append(args, "xadd", a.Stream)
	if a.NoMkStream {
		args = append(args, "nomkstream")
	}
	switch {
	case a.MaxLen > 0:
		if a.Approx {
			args = append(args, "maxlen", "~", a.MaxLen)
		} else {
			args = append(args, "maxlen", a.MaxLen)
		}
	case a.MinID != "":
		if a.Approx {
			args = append(args, "minid", "~", a.MinID)
		} else {
			args = append(args, "minid", a.MinID)
		}
	}
	if a.Limit > 0 {
		args = append(args, "limit", a.Limit)
	}
	if a.ID != "" {
		args = append(args, a.ID)
	} else {
		args = append(args, "*")
	}
	args = appendArg(args, a.Values)

	cmd := NewStringCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XDel(ctx context.Context, stream string, ids ...string) *IntCmd {
	args := []interface{}{"xdel", stream}
	for _, id := range ids {
		args = append(args, id)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XLen(ctx context.Context, stream string) *IntCmd {
	cmd := NewIntCmd(ctx, "xlen", stream)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XRange(ctx context.Context, stream, start, stop string) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd(ctx, "xrange", stream, start, stop)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XRangeN(ctx context.Context, stream, start, stop string, count int64) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd(ctx, "xrange", stream, start, stop, "count", count)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XRevRange(ctx context.Context, stream, start, stop string) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd(ctx, "xrevrange", stream, start, stop)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XRevRangeN(ctx context.Context, stream, start, stop string, count int64) *XMessageSliceCmd {
	cmd := NewXMessageSliceCmd(ctx, "xrevrange", stream, start, stop, "count", count)
	_ = c(ctx, cmd)
	return cmd
}

type XReadArgs struct {
	Streams []string // list of streams and ids, e.g. stream1 stream2 id1 id2
	Count   int64
	Block   time.Duration
}

func (c cmdable) XRead(ctx context.Context, a *XReadArgs) *XStreamSliceCmd {
	args := make([]interface{}, 0, 6+len(a.Streams))
	args = append(args, "xread")

	keyPos := int8(1)
	if a.Count > 0 {
		args = append(args, "count")
		args = append(args, a.Count)
		keyPos += 2
	}
	if a.Block >= 0 {
		args = append(args, "block")
		args = append(args, int64(a.Block/time.Millisecond))
		keyPos += 2
	}
	args = append(args, "streams")
	keyPos++
	for _, s := range a.Streams {
		args = append(args, s)
	}

	cmd := NewXStreamSliceCmd(ctx, args...)
	if a.Block >= 0 {
		cmd.setReadTimeout(a.Block)
	}
	cmd.SetFirstKeyPos(keyPos)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XReadStreams(ctx context.Context, streams ...string) *XStreamSliceCmd {
	return c.XRead(ctx, &XReadArgs{
		Streams: streams,
		Block:   -1,
	})
}

func (c cmdable) XGroupCreate(ctx context.Context, stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "xgroup", "create", stream, group, start)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "xgroup", "create", stream, group, start, "mkstream")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XGroupSetID(ctx context.Context, stream, group, start string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "xgroup", "setid", stream, group, start)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XGroupDestroy(ctx context.Context, stream, group string) *IntCmd {
	cmd := NewIntCmd(ctx, "xgroup", "destroy", stream, group)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XGroupCreateConsumer(ctx context.Context, stream, group, consumer string) *IntCmd {
	cmd := NewIntCmd(ctx, "xgroup", "createconsumer", stream, group, consumer)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XGroupDelConsumer(ctx context.Context, stream, group, consumer string) *IntCmd {
	cmd := NewIntCmd(ctx, "xgroup", "delconsumer", stream, group, consumer)
	_ = c(ctx, cmd)
	return cmd
}

type XReadGroupArgs struct {
	Group    string
	Consumer string
	Streams  []string // list of streams and ids, e.g. stream1 stream2 id1 id2
	Count    int64
	Block    time.Duration
	NoAck    bool
}

func (c cmdable) XReadGroup(ctx context.Context, a *XReadGroupArgs) *XStreamSliceCmd {
	args := make([]interface{}, 0, 10+len(a.Streams))
	args = append(args, "xreadgroup", "group", a.Group, a.Consumer)

	keyPos := int8(4)
	if a.Count > 0 {
		args = append(args, "count", a.Count)
		keyPos += 2
	}
	if a.Block >= 0 {
		args = append(args, "block", int64(a.Block/time.Millisecond))
		keyPos += 2
	}
	if a.NoAck {
		args = append(args, "noack")
		keyPos++
	}
	args = append(args, "streams")
	keyPos++
	for _, s := range a.Streams {
		args = append(args, s)
	}

	cmd := NewXStreamSliceCmd(ctx, args...)
	if a.Block >= 0 {
		cmd.setReadTimeout(a.Block)
	}
	cmd.SetFirstKeyPos(keyPos)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XAck(ctx context.Context, stream, group string, ids ...string) *IntCmd {
	args := []interface{}{"xack", stream, group}
	for _, id := range ids {
		args = append(args, id)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XPending(ctx context.Context, stream, group string) *XPendingCmd {
	cmd := NewXPendingCmd(ctx, "xpending", stream, group)
	_ = c(ctx, cmd)
	return cmd
}

type XPendingExtArgs struct {
	Stream   string
	Group    string
	Idle     time.Duration
	Start    string
	End      string
	Count    int64
	Consumer string
}

func (c cmdable) XPendingExt(ctx context.Context, a *XPendingExtArgs) *XPendingExtCmd {
	args := make([]interface{}, 0, 9)
	args = append(args, "xpending", a.Stream, a.Group)
	if a.Idle != 0 {
		args = append(args, "idle", formatMs(ctx, a.Idle))
	}
	args = append(args, a.Start, a.End, a.Count)
	if a.Consumer != "" {
		args = append(args, a.Consumer)
	}
	cmd := NewXPendingExtCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

type XAutoClaimArgs struct {
	Stream   string
	Group    string
	MinIdle  time.Duration
	Start    string
	Count    int64
	Consumer string
}

func (c cmdable) XAutoClaim(ctx context.Context, a *XAutoClaimArgs) *XAutoClaimCmd {
	args := xAutoClaimArgs(ctx, a)
	cmd := NewXAutoClaimCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XAutoClaimJustID(ctx context.Context, a *XAutoClaimArgs) *XAutoClaimJustIDCmd {
	args := xAutoClaimArgs(ctx, a)
	args = append(args, "justid")
	cmd := NewXAutoClaimJustIDCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func xAutoClaimArgs(ctx context.Context, a *XAutoClaimArgs) []interface{} {
	args := make([]interface{}, 0, 8)
	args = append(args, "xautoclaim", a.Stream, a.Group, a.Consumer, formatMs(ctx, a.MinIdle), a.Start)
	if a.Count > 0 {
		args = append(args, "count", a.Count)
	}
	return args
}

type XClaimArgs struct {
	Stream   string
	Group    string
	Consumer string
	MinIdle  time.Duration
	Messages []string
}

func (c cmdable) XClaim(ctx context.Context, a *XClaimArgs) *XMessageSliceCmd {
	args := xClaimArgs(a)
	cmd := NewXMessageSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XClaimJustID(ctx context.Context, a *XClaimArgs) *StringSliceCmd {
	args := xClaimArgs(a)
	args = append(args, "justid")
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func xClaimArgs(a *XClaimArgs) []interface{} {
	args := make([]interface{}, 0, 5+len(a.Messages))
	args = append(args,
		"xclaim",
		a.Stream,
		a.Group, a.Consumer,
		int64(a.MinIdle/time.Millisecond))
	for _, id := range a.Messages {
		args = append(args, id)
	}
	return args
}

// xTrim If approx is true, add the "~" parameter, otherwise it is the default "=" (redis default).
// example:
//
//	XTRIM key MAXLEN/MINID threshold LIMIT limit.
//	XTRIM key MAXLEN/MINID ~ threshold LIMIT limit.
//
// The redis-server version is lower than 6.2, please set limit to 0.
func (c cmdable) xTrim(
	ctx context.Context, key, strategy string,
	approx bool, threshold interface{}, limit int64,
) *IntCmd {
	args := make([]interface{}, 0, 7)
	args = append(args, "xtrim", key, strategy)
	if approx {
		args = append(args, "~")
	}
	args = append(args, threshold)
	if limit > 0 {
		args = append(args, "limit", limit)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// XTrimMaxLen No `~` rules are used, `limit` cannot be used.
// cmd: XTRIM key MAXLEN maxLen
func (c cmdable) XTrimMaxLen(ctx context.Context, key string, maxLen int64) *IntCmd {
	return c.xTrim(ctx, key, "maxlen", false, maxLen, 0)
}

func (c cmdable) XTrimMaxLenApprox(ctx context.Context, key string, maxLen, limit int64) *IntCmd {
	return c.xTrim(ctx, key, "maxlen", true, maxLen, limit)
}

func (c cmdable) XTrimMinID(ctx context.Context, key string, minID string) *IntCmd {
	return c.xTrim(ctx, key, "minid", false, minID, 0)
}

func (c cmdable) XTrimMinIDApprox(ctx context.Context, key string, minID string, limit int64) *IntCmd {
	return c.xTrim(ctx, key, "minid", true, minID, limit)
}

func (c cmdable) XInfoConsumers(ctx context.Context, key string, group string) *XInfoConsumersCmd {
	cmd := NewXInfoConsumersCmd(ctx, key, group)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XInfoGroups(ctx context.Context, key string) *XInfoGroupsCmd {
	cmd := NewXInfoGroupsCmd(ctx, key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) XInfoStream(ctx context.Context, key string) *XInfoStreamCmd {
	cmd := NewXInfoStreamCmd(ctx, key)
	_ = c(ctx, cmd)
	return cmd
}

// XInfoStreamFull XINFO STREAM FULL [COUNT count]
// redis-server >= 6.0.
func (c cmdable) XInfoStreamFull(ctx context.Context, key string, count int) *XInfoStreamFullCmd {
	args := make([]interface{}, 0, 6)
	args = append(args, "xinfo", "stream", key, "full")
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := NewXInfoStreamFullCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

// Z represents sorted set member.
type Z struct {
	Score  float64
	Member interface{}
}

// ZWithKey represents sorted set member including the name of the key where it was popped.
type ZWithKey struct {
	Z
	Key string
}

// ZStore is used as an arg to ZInter/ZInterStore and ZUnion/ZUnionStore.
type ZStore struct {
	Keys    []string
	Weights []float64
	// Can be SUM, MIN or MAX.
	Aggregate string
}

func (z ZStore) len() (n int) {
	n = len(z.Keys)
	if len(z.Weights) > 0 {
		n += 1 + len(z.Weights)
	}
	if z.Aggregate != "" {
		n += 2
	}
	return n
}

func (z ZStore) appendArgs(args []interface{}) []interface{} {
	for _, key := range z.Keys {
		args = append(args, key)
	}
	if len(z.Weights) > 0 {
		args = append(args, "weights")
		for _, weights := range z.Weights {
			args = append(args, weights)
		}
	}
	if z.Aggregate != "" {
		args = append(args, "aggregate", z.Aggregate)
	}
	return args
}

// BZPopMax Redis `BZPOPMAX key [key ...] timeout` command.
func (c cmdable) BZPopMax(ctx context.Context, timeout time.Duration, keys ...string) *ZWithKeyCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "bzpopmax"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(ctx, timeout)
	cmd := NewZWithKeyCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

// BZPopMin Redis `BZPOPMIN key [key ...] timeout` command.
func (c cmdable) BZPopMin(ctx context.Context, timeout time.Duration, keys ...string) *ZWithKeyCmd {
	args := make([]interface{}, 1+len(keys)+1)
	args[0] = "bzpopmin"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(ctx, timeout)
	cmd := NewZWithKeyCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

// BZMPop is the blocking variant of ZMPOP.
// When any of the sorted sets contains elements, this command behaves exactly like ZMPOP.
// When all sorted sets are empty, Redis will block the connection until another client adds members to one of the keys or until the timeout elapses.
// A timeout of zero can be used to block indefinitely.
// example: client.BZMPop(ctx, 0,"max", 1, "set")
func (c cmdable) BZMPop(ctx context.Context, timeout time.Duration, order string, count int64, keys ...string) *ZSliceWithKeyCmd {
	args := make([]interface{}, 3+len(keys), 6+len(keys))
	args[0] = "bzmpop"
	args[1] = formatSec(ctx, timeout)
	args[2] = len(keys)
	for i, key := range keys {
		args[3+i] = key
	}
	args = append(args, strings.ToLower(order), "count", count)
	cmd := NewZSliceWithKeyCmd(ctx, args...)
	cmd.setReadTimeout(timeout)
	_ = c(ctx, cmd)
	return cmd
}

// ZAddArgs WARN: The GT, LT and NX options are mutually exclusive.
type ZAddArgs struct {
	NX      bool
	XX      bool
	LT      bool
	GT      bool
	Ch      bool
	Members []Z
}

func (c cmdable) zAddArgs(key string, args ZAddArgs, incr bool) []interface{} {
	a := make([]interface{}, 0, 6+2*len(args.Members))
	a = append(a, "zadd", key)

	// The GT, LT and NX options are mutually exclusive.
	if args.NX {
		a = append(a, "nx")
	} else {
		if args.XX {
			a = append(a, "xx")
		}
		if args.GT {
			a = append(a, "gt")
		} else if args.LT {
			a = append(a, "lt")
		}
	}
	if args.Ch {
		a = append(a, "ch")
	}
	if incr {
		a = append(a, "incr")
	}
	for _, m := range args.Members {
		a = append(a, m.Score)
		a = append(a, m.Member)
	}
	return a
}

func (c cmdable) ZAddArgs(ctx context.Context, key string, args ZAddArgs) *IntCmd {
	cmd := NewIntCmd(ctx, c.zAddArgs(key, args, false)...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZAddArgsIncr(ctx context.Context, key string, args ZAddArgs) *FloatCmd {
	cmd := NewFloatCmd(ctx, c.zAddArgs(key, args, true)...)
	_ = c(ctx, cmd)
	return cmd
}

// ZAdd Redis `ZADD key score member [score member ...]` command.
func (c cmdable) ZAdd(ctx context.Context, key string, members ...Z) *IntCmd {
	return c.ZAddArgs(ctx, key, ZAddArgs{
		Members: members,
	})
}

// ZAddLT Redis `ZADD key LT score member [score member ...]` command.
func (c cmdable) ZAddLT(ctx context.Context, key string, members ...Z) *IntCmd {
	return c.ZAddArgs(ctx, key, ZAddArgs{
		LT:      true,
		Members: members,
	})
}

// ZAddGT Redis `ZADD key GT score member [score member ...]` command.
func (c cmdable) ZAddGT(ctx context.Context, key string, members ...Z) *IntCmd {
	return c.ZAddArgs(ctx, key, ZAddArgs{
		GT:      true,
		Members: members,
	})
}

// ZAddNX Redis `ZADD key NX score member [score member ...]` command.
func (c cmdable) ZAddNX(ctx context.Context, key string, members ...Z) *IntCmd {
	return c.ZAddArgs(ctx, key, ZAddArgs{
		NX:      true,
		Members: members,
	})
}

// ZAddXX Redis `ZADD key XX score member [score member ...]` command.
func (c cmdable) ZAddXX(ctx context.Context, key string, members ...Z) *IntCmd {
	return c.ZAddArgs(ctx, key, ZAddArgs{
		XX:      true,
		Members: members,
	})
}

func (c cmdable) ZCard(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "zcard", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZCount(ctx context.Context, key, min, max string) *IntCmd {
	cmd := NewIntCmd(ctx, "zcount", key, min, max)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZLexCount(ctx context.Context, key, min, max string) *IntCmd {
	cmd := NewIntCmd(ctx, "zlexcount", key, min, max)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZIncrBy(ctx context.Context, key string, increment float64, member string) *FloatCmd {
	cmd := NewFloatCmd(ctx, "zincrby", key, increment, member)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZInterStore(ctx context.Context, destination string, store *ZStore) *IntCmd {
	args := make([]interface{}, 0, 3+store.len())
	args = append(args, "zinterstore", destination, len(store.Keys))
	args = store.appendArgs(args)
	cmd := NewIntCmd(ctx, args...)
	cmd.SetFirstKeyPos(3)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZInter(ctx context.Context, store *ZStore) *StringSliceCmd {
	args := make([]interface{}, 0, 2+store.len())
	args = append(args, "zinter", len(store.Keys))
	args = store.appendArgs(args)
	cmd := NewStringSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZInterWithScores(ctx context.Context, store *ZStore) *ZSliceCmd {
	args := make([]interface{}, 0, 3+store.len())
	args = append(args, "zinter", len(store.Keys))
	args = store.appendArgs(args)
	args = append(args, "withscores")
	cmd := NewZSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZInterCard(ctx context.Context, limit int64, keys ...string) *IntCmd {
	args := make([]interface{}, 4+len(keys))
	args[0] = "zintercard"
	numkeys := int64(0)
	for i, key := range keys {
		args[2+i] = key
		numkeys++
	}
	args[1] = numkeys
	args[2+numkeys] = "limit"
	args[3+numkeys] = limit
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// ZMPop Pops one or more elements with the highest or lowest score from the first non-empty sorted set key from the list of provided key names.
// direction: "max" (highest score) or "min" (lowest score), count: > 0
// example: client.ZMPop(ctx, "max", 5, "set1", "set2")
func (c cmdable) ZMPop(ctx context.Context, order string, count int64, keys ...string) *ZSliceWithKeyCmd {
	args := make([]interface{}, 2+len(keys), 5+len(keys))
	args[0] = "zmpop"
	args[1] = len(keys)
	for i, key := range keys {
		args[2+i] = key
	}
	args = append(args, strings.ToLower(order), "count", count)
	cmd := NewZSliceWithKeyCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZMScore(ctx context.Context, key string, members ...string) *FloatSliceCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "zmscore"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewFloatSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZPopMax(ctx context.Context, key string, count ...int64) *ZSliceCmd {
	args := []interface{}{
		"zpopmax",
		key,
	}

	switch len(count) {
	case 0:
		break
	case 1:
		args = append(args, count[0])
	default:
		panic("too many arguments")
	}

	cmd := NewZSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZPopMin(ctx context.Context, key string, count ...int64) *ZSliceCmd {
	args := []interface{}{
		"zpopmin",
		key,
	}

	switch len(count) {
	case 0:
		break
	case 1:
		args = append(args, count[0])
	default:
		panic("too many arguments")
	}

	cmd := NewZSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// ZRangeArgs is all the options of the ZRange command.
// In version> 6.2.0, you can replace the(cmd):
//
//	ZREVRANGE,
//	ZRANGEBYSCORE,
//	ZREVRANGEBYSCORE,
//	ZRANGEBYLEX,
//	ZREVRANGEBYLEX.
//
// Please pay attention to your redis-server version.
//
// Rev, ByScore, ByLex and Offset+Count options require redis-server 6.2.0 and higher.
type ZRangeArgs struct {
	Key string

	// When the ByScore option is provided, the open interval(exclusive) can be set.
	// By default, the score intervals specified by <Start> and <Stop> are closed (inclusive).
	// It is similar to the deprecated(6.2.0+) ZRangeByScore command.
	// For example:
	//		ZRangeArgs{
	//			Key: 				"example-key",
	//	 		Start: 				"(3",
	//	 		Stop: 				8,
	//			ByScore:			true,
	//	 	}
	// 	 	cmd: "ZRange example-key (3 8 ByScore"  (3 < score <= 8).
	//
	// For the ByLex option, it is similar to the deprecated(6.2.0+) ZRangeByLex command.
	// You can set the <Start> and <Stop> options as follows:
	//		ZRangeArgs{
	//			Key: 				"example-key",
	//	 		Start: 				"[abc",
	//	 		Stop: 				"(def",
	//			ByLex:				true,
	//	 	}
	//		cmd: "ZRange example-key [abc (def ByLex"
	//
	// For normal cases (ByScore==false && ByLex==false), <Start> and <Stop> should be set to the index range (int).
	// You can read the documentation for more information: https://redis.io/commands/zrange
	Start interface{}
	Stop  interface{}

	// The ByScore and ByLex options are mutually exclusive.
	ByScore bool
	ByLex   bool

	Rev bool

	// limit offset count.
	Offset int64
	Count  int64
}

func (z ZRangeArgs) appendArgs(args []interface{}) []interface{} {
	// For Rev+ByScore/ByLex, we need to adjust the position of <Start> and <Stop>.
	if z.Rev && (z.ByScore || z.ByLex) {
		args = append(args, z.Key, z.Stop, z.Start)
	} else {
		args = append(args, z.Key, z.Start, z.Stop)
	}

	if z.ByScore {
		args = append(args, "byscore")
	} else if z.ByLex {
		args = append(args, "bylex")
	}
	if z.Rev {
		args = append(args, "rev")
	}
	if z.Offset != 0 || z.Count != 0 {
		args = append(args, "limit", z.Offset, z.Count)
	}
	return args
}

func (c cmdable) ZRangeArgs(ctx context.Context, z ZRangeArgs) *StringSliceCmd {
	args := make([]interface{}, 0, 9)
	args = append(args, "zrange")
	args = z.appendArgs(args)
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRangeArgsWithScores(ctx context.Context, z ZRangeArgs) *ZSliceCmd {
	args := make([]interface{}, 0, 10)
	args = append(args, "zrange")
	args = z.appendArgs(args)
	args = append(args, "withscores")
	cmd := NewZSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd {
	return c.ZRangeArgs(ctx, ZRangeArgs{
		Key:   key,
		Start: start,
		Stop:  stop,
	})
}

func (c cmdable) ZRangeWithScores(ctx context.Context, key string, start, stop int64) *ZSliceCmd {
	return c.ZRangeArgsWithScores(ctx, ZRangeArgs{
		Key:   key,
		Start: start,
		Stop:  stop,
	})
}

type ZRangeBy struct {
	Min, Max      string
	Offset, Count int64
}

func (c cmdable) zRangeBy(ctx context.Context, zcmd, key string, opt *ZRangeBy, withScores bool) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Min, opt.Max}
	if withScores {
		args = append(args, "withscores")
	}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRangeBy(ctx, "zrangebyscore", key, opt, false)
}

func (c cmdable) ZRangeByLex(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRangeBy(ctx, "zrangebylex", key, opt, false)
}

func (c cmdable) ZRangeByScoreWithScores(ctx context.Context, key string, opt *ZRangeBy) *ZSliceCmd {
	args := []interface{}{"zrangebyscore", key, opt.Min, opt.Max, "withscores"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRangeStore(ctx context.Context, dst string, z ZRangeArgs) *IntCmd {
	args := make([]interface{}, 0, 10)
	args = append(args, "zrangestore", dst)
	args = z.appendArgs(args)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRank(ctx context.Context, key, member string) *IntCmd {
	cmd := NewIntCmd(ctx, "zrank", key, member)
	_ = c(ctx, cmd)
	return cmd
}

// ZRankWithScore according to the Redis documentation, if member does not exist
// in the sorted set or key does not exist, it will return a redis.Nil error.
func (c cmdable) ZRankWithScore(ctx context.Context, key, member string) *RankWithScoreCmd {
	cmd := NewRankWithScoreCmd(ctx, "zrank", key, member, "withscore")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(members))
	args[0] = "zrem"
	args[1] = key
	args = appendArgs(args, members)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *IntCmd {
	cmd := NewIntCmd(
		ctx,
		"zremrangebyrank",
		key,
		start,
		stop,
	)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRemRangeByScore(ctx context.Context, key, min, max string) *IntCmd {
	cmd := NewIntCmd(ctx, "zremrangebyscore", key, min, max)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRemRangeByLex(ctx context.Context, key, min, max string) *IntCmd {
	cmd := NewIntCmd(ctx, "zremrangebylex", key, min, max)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRevRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "zrevrange", key, start, stop)
	_ = c(ctx, cmd)
	return cmd
}

// ZRevRangeWithScores according to the Redis documentation, if member does not exist
// in the sorted set or key does not exist, it will return a redis.Nil error.
func (c cmdable) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *ZSliceCmd {
	cmd := NewZSliceCmd(ctx, "zrevrange", key, start, stop, "withscores")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) zRevRangeBy(ctx context.Context, zcmd, key string, opt *ZRangeBy) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Max, opt.Min}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRevRangeByScore(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRevRangeBy(ctx, "zrevrangebyscore", key, opt)
}

func (c cmdable) ZRevRangeByLex(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd {
	return c.zRevRangeBy(ctx, "zrevrangebylex", key, opt)
}

func (c cmdable) ZRevRangeByScoreWithScores(ctx context.Context, key string, opt *ZRangeBy) *ZSliceCmd {
	args := []interface{}{"zrevrangebyscore", key, opt.Max, opt.Min, "withscores"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"limit",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRevRank(ctx context.Context, key, member string) *IntCmd {
	cmd := NewIntCmd(ctx, "zrevrank", key, member)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZRevRankWithScore(ctx context.Context, key, member string) *RankWithScoreCmd {
	cmd := NewRankWithScoreCmd(ctx, "zrevrank", key, member, "withscore")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZScore(ctx context.Context, key, member string) *FloatCmd {
	cmd := NewFloatCmd(ctx, "zscore", key, member)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZUnion(ctx context.Context, store ZStore) *StringSliceCmd {
	args := make([]interface{}, 0, 2+store.len())
	args = append(args, "zunion", len(store.Keys))
	args = store.appendArgs(args)
	cmd := NewStringSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZUnionWithScores(ctx context.Context, store ZStore) *ZSliceCmd {
	args := make([]interface{}, 0, 3+store.len())
	args = append(args, "zunion", len(store.Keys))
	args = store.appendArgs(args)
	args = append(args, "withscores")
	cmd := NewZSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ZUnionStore(ctx context.Context, dest string, store *ZStore) *IntCmd {
	args := make([]interface{}, 0, 3+store.len())
	args = append(args, "zunionstore", dest, len(store.Keys))
	args = store.appendArgs(args)
	cmd := NewIntCmd(ctx, args...)
	cmd.SetFirstKeyPos(3)
	_ = c(ctx, cmd)
	return cmd
}

// ZRandMember redis-server version >= 6.2.0.
func (c cmdable) ZRandMember(ctx context.Context, key string, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "zrandmember", key, count)
	_ = c(ctx, cmd)
	return cmd
}

// ZRandMemberWithScores redis-server version >= 6.2.0.
func (c cmdable) ZRandMemberWithScores(ctx context.Context, key string, count int) *ZSliceCmd {
	cmd := NewZSliceCmd(ctx, "zrandmember", key, count, "withscores")
	_ = c(ctx, cmd)
	return cmd
}

// ZDiff redis-server version >= 6.2.0.
func (c cmdable) ZDiff(ctx context.Context, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "zdiff"
	args[1] = len(keys)
	for i, key := range keys {
		args[i+2] = key
	}

	cmd := NewStringSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

// ZDiffWithScores redis-server version >= 6.2.0.
func (c cmdable) ZDiffWithScores(ctx context.Context, keys ...string) *ZSliceCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "zdiff"
	args[1] = len(keys)
	for i, key := range keys {
		args[i+2] = key
	}
	args[len(keys)+2] = "withscores"

	cmd := NewZSliceCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

// ZDiffStore redis-server version >=6.2.0.
func (c cmdable) ZDiffStore(ctx context.Context, destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 0, 3+len(keys))
	args = append(args, "zdiffstore", destination, len(keys))
	for _, key := range keys {
		args = append(args, key)
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) PFAdd(ctx context.Context, key string, els ...interface{}) *IntCmd {
	args := make([]interface{}, 2, 2+len(els))
	args[0] = "pfadd"
	args[1] = key
	args = appendArgs(args, els)
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PFCount(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "pfcount"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PFMerge(ctx context.Context, dest string, keys ...string) *StatusCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "pfmerge"
	args[1] = dest
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) BgRewriteAOF(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "bgrewriteaof")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) BgSave(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "bgsave")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientKill(ctx context.Context, ipPort string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "client", "kill", ipPort)
	_ = c(ctx, cmd)
	return cmd
}

// ClientKillByFilter is new style syntax, while the ClientKill is old
//
//	CLIENT KILL <option> [value] ... <option> [value]
func (c cmdable) ClientKillByFilter(ctx context.Context, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "client"
	args[1] = "kill"
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientList(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "client", "list")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientPause(ctx context.Context, dur time.Duration) *BoolCmd {
	cmd := NewBoolCmd(ctx, "client", "pause", formatMs(ctx, dur))
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientUnpause(ctx context.Context) *BoolCmd {
	cmd := NewBoolCmd(ctx, "client", "unpause")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientID(ctx context.Context) *IntCmd {
	cmd := NewIntCmd(ctx, "client", "id")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientUnblock(ctx context.Context, id int64) *IntCmd {
	cmd := NewIntCmd(ctx, "client", "unblock", id)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientUnblockWithError(ctx context.Context, id int64) *IntCmd {
	cmd := NewIntCmd(ctx, "client", "unblock", id, "error")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClientInfo(ctx context.Context) *ClientInfoCmd {
	cmd := NewClientInfoCmd(ctx, "client", "info")
	_ = c(ctx, cmd)
	return cmd
}

// ------------------------------------------------------------------------------------------------

func (c cmdable) ConfigGet(ctx context.Context, parameter string) *MapStringStringCmd {
	cmd := NewMapStringStringCmd(ctx, "config", "get", parameter)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ConfigResetStat(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "config", "resetstat")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ConfigSet(ctx context.Context, parameter, value string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "config", "set", parameter, value)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ConfigRewrite(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "config", "rewrite")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) DBSize(ctx context.Context) *IntCmd {
	cmd := NewIntCmd(ctx, "dbsize")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FlushAll(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "flushall")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FlushAllAsync(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "flushall", "async")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FlushDB(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "flushdb")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FlushDBAsync(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "flushdb", "async")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Info(ctx context.Context, sections ...string) *StringCmd {
	args := make([]interface{}, 1+len(sections))
	args[0] = "info"
	for i, section := range sections {
		args[i+1] = section
	}
	cmd := NewStringCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) LastSave(ctx context.Context) *IntCmd {
	cmd := NewIntCmd(ctx, "lastsave")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Save(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "save")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) shutdown(ctx context.Context, modifier string) *StatusCmd {
	var args []interface{}
	if modifier == "" {
		args = []interface{}{"shutdown"}
	} else {
		args = []interface{}{"shutdown", modifier}
	}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	if err := cmd.Err(); err != nil {
		if err == io.EOF {
			// Server quit as expected.
			cmd.err = nil
		}
	} else {
		// Server did not quit. String reply contains the reason.
		cmd.err = errors.New(cmd.val)
		cmd.val = ""
	}
	return cmd
}

func (c cmdable) Shutdown(ctx context.Context) *StatusCmd {
	return c.shutdown(ctx, "")
}

func (c cmdable) ShutdownSave(ctx context.Context) *StatusCmd {
	return c.shutdown(ctx, "save")
}

func (c cmdable) ShutdownNoSave(ctx context.Context) *StatusCmd {
	return c.shutdown(ctx, "nosave")
}

func (c cmdable) SlaveOf(ctx context.Context, host, port string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "slaveof", host, port)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SlowLogGet(ctx context.Context, num int64) *SlowLogCmd {
	cmd := NewSlowLogCmd(context.Background(), "slowlog", "get", num)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) Sync(_ context.Context) {
	panic("not implemented")
}

func (c cmdable) Time(ctx context.Context) *TimeCmd {
	cmd := NewTimeCmd(ctx, "time")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) DebugObject(ctx context.Context, key string) *StringCmd {
	cmd := NewStringCmd(ctx, "debug", "object", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ReadOnly(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "readonly")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ReadWrite(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "readwrite")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) MemoryUsage(ctx context.Context, key string, samples ...int) *IntCmd {
	args := []interface{}{"memory", "usage", key}
	if len(samples) > 0 {
		if len(samples) != 1 {
			panic("MemoryUsage expects single sample count")
		}
		args = append(args, "SAMPLES", samples[0])
	}
	cmd := NewIntCmd(ctx, args...)
	cmd.SetFirstKeyPos(2)
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *Cmd {
	return c.eval(ctx, "eval", script, keys, args...)
}

func (c cmdable) EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) *Cmd {
	return c.eval(ctx, "eval_ro", script, keys, args...)
}

func (c cmdable) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *Cmd {
	return c.eval(ctx, "evalsha", sha1, keys, args...)
}

func (c cmdable) EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) *Cmd {
	return c.eval(ctx, "evalsha_ro", sha1, keys, args...)
}

func (c cmdable) eval(ctx context.Context, name, payload string, keys []string, args ...interface{}) *Cmd {
	cmdArgs := make([]interface{}, 3+len(keys), 3+len(keys)+len(args))
	cmdArgs[0] = name
	cmdArgs[1] = payload
	cmdArgs[2] = len(keys)
	for i, key := range keys {
		cmdArgs[3+i] = key
	}
	cmdArgs = appendArgs(cmdArgs, args)
	cmd := NewCmd(ctx, cmdArgs...)

	// it is possible that only args exist without a key.
	// rdb.eval(ctx, eval, script, nil, arg1, arg2)
	if len(keys) > 0 {
		cmd.SetFirstKeyPos(3)
	}
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ScriptExists(ctx context.Context, hashes ...string) *BoolSliceCmd {
	args := make([]interface{}, 2+len(hashes))
	args[0] = "script"
	args[1] = "exists"
	for i, hash := range hashes {
		args[2+i] = hash
	}
	cmd := NewBoolSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ScriptFlush(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "script", "flush")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ScriptKill(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "script", "kill")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ScriptLoad(ctx context.Context, script string) *StringCmd {
	cmd := NewStringCmd(ctx, "script", "load", script)
	_ = c(ctx, cmd)
	return cmd
}

// ------------------------------------------------------------------------------

// FunctionListQuery is used with FunctionList to query for Redis libraries
//
//	  	LibraryNamePattern 	- Use an empty string to get all libraries.
//	  						- Use a glob-style pattern to match multiple libraries with a matching name
//	  						- Use a library's full name to match a single library
//		WithCode			- If true, it will return the code of the library
type FunctionListQuery struct {
	LibraryNamePattern string
	WithCode           bool
}

func (c cmdable) FunctionLoad(ctx context.Context, code string) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "load", code)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionLoadReplace(ctx context.Context, code string) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "load", "replace", code)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionDelete(ctx context.Context, libName string) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "delete", libName)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionFlush(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "flush")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionKill(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "kill")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionFlushAsync(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "flush", "async")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionList(ctx context.Context, q FunctionListQuery) *FunctionListCmd {
	args := make([]interface{}, 2, 5)
	args[0] = "function"
	args[1] = "list"
	if q.LibraryNamePattern != "" {
		args = append(args, "libraryname", q.LibraryNamePattern)
	}
	if q.WithCode {
		args = append(args, "withcode")
	}
	cmd := NewFunctionListCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionDump(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "dump")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionRestore(ctx context.Context, libDump string) *StringCmd {
	cmd := NewStringCmd(ctx, "function", "restore", libDump)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FunctionStats(ctx context.Context) *FunctionStatsCmd {
	cmd := NewFunctionStatsCmd(ctx, "function", "stats")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) FCall(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd {
	cmdArgs := fcallArgs("fcall", function, keys, args...)
	cmd := NewCmd(ctx, cmdArgs...)
	if len(keys) > 0 {
		cmd.SetFirstKeyPos(3)
	}
	_ = c(ctx, cmd)
	return cmd
}

// FCallRo this function simply calls FCallRO,
// Deprecated: to maintain convention FCallRO.
func (c cmdable) FCallRo(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd {
	return c.FCallRO(ctx, function, keys, args...)
}

func (c cmdable) FCallRO(ctx context.Context, function string, keys []string, args ...interface{}) *Cmd {
	cmdArgs := fcallArgs("fcall_ro", function, keys, args...)
	cmd := NewCmd(ctx, cmdArgs...)
	if len(keys) > 0 {
		cmd.SetFirstKeyPos(3)
	}
	_ = c(ctx, cmd)
	return cmd
}

func fcallArgs(command string, function string, keys []string, args ...interface{}) []interface{} {
	cmdArgs := make([]interface{}, 3+len(keys), 3+len(keys)+len(args))
	cmdArgs[0] = command
	cmdArgs[1] = function
	cmdArgs[2] = len(keys)
	for i, key := range keys {
		cmdArgs[3+i] = key
	}

	cmdArgs = append(cmdArgs, args...)
	return cmdArgs
}

//------------------------------------------------------------------------------

// Publish posts the message to the channel.
func (c cmdable) Publish(ctx context.Context, channel string, message interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "publish", channel, message)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) SPublish(ctx context.Context, channel string, message interface{}) *IntCmd {
	cmd := NewIntCmd(ctx, "spublish", channel, message)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PubSubChannels(ctx context.Context, pattern string) *StringSliceCmd {
	args := []interface{}{"pubsub", "channels"}
	if pattern != "*" {
		args = append(args, pattern)
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PubSubNumSub(ctx context.Context, channels ...string) *MapStringIntCmd {
	args := make([]interface{}, 2+len(channels))
	args[0] = "pubsub"
	args[1] = "numsub"
	for i, channel := range channels {
		args[2+i] = channel
	}
	cmd := NewMapStringIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PubSubShardChannels(ctx context.Context, pattern string) *StringSliceCmd {
	args := []interface{}{"pubsub", "shardchannels"}
	if pattern != "*" {
		args = append(args, pattern)
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PubSubShardNumSub(ctx context.Context, channels ...string) *MapStringIntCmd {
	args := make([]interface{}, 2+len(channels))
	args[0] = "pubsub"
	args[1] = "shardnumsub"
	for i, channel := range channels {
		args[2+i] = channel
	}
	cmd := NewMapStringIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) PubSubNumPat(ctx context.Context) *IntCmd {
	cmd := NewIntCmd(ctx, "pubsub", "numpat")
	_ = c(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c cmdable) ClusterMyShardID(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "cluster", "myshardid")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterSlots(ctx context.Context) *ClusterSlotsCmd {
	cmd := NewClusterSlotsCmd(ctx, "cluster", "slots")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterShards(ctx context.Context) *ClusterShardsCmd {
	cmd := NewClusterShardsCmd(ctx, "cluster", "shards")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterLinks(ctx context.Context) *ClusterLinksCmd {
	cmd := NewClusterLinksCmd(ctx, "cluster", "links")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterNodes(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "cluster", "nodes")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterMeet(ctx context.Context, host, port string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "meet", host, port)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterForget(ctx context.Context, nodeID string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "forget", nodeID)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterReplicate(ctx context.Context, nodeID string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "replicate", nodeID)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterResetSoft(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "reset", "soft")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterResetHard(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "reset", "hard")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterInfo(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "cluster", "info")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterKeySlot(ctx context.Context, key string) *IntCmd {
	cmd := NewIntCmd(ctx, "cluster", "keyslot", key)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterGetKeysInSlot(ctx context.Context, slot int, count int) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "cluster", "getkeysinslot", slot, count)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterCountFailureReports(ctx context.Context, nodeID string) *IntCmd {
	cmd := NewIntCmd(ctx, "cluster", "count-failure-reports", nodeID)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterCountKeysInSlot(ctx context.Context, slot int) *IntCmd {
	cmd := NewIntCmd(ctx, "cluster", "countkeysinslot", slot)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterDelSlots(ctx context.Context, slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "cluster"
	args[1] = "delslots"
	for i, slot := range slots {
		args[2+i] = slot
	}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterDelSlotsRange(ctx context.Context, min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterDelSlots(ctx, slots...)
}

func (c cmdable) ClusterSaveConfig(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "saveconfig")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterSlaves(ctx context.Context, nodeID string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "cluster", "slaves", nodeID)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterFailover(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "cluster", "failover")
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterAddSlots(ctx context.Context, slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "cluster"
	args[1] = "addslots"
	for i, num := range slots {
		args[2+i] = num
	}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ClusterAddSlotsRange(ctx context.Context, min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterAddSlots(ctx, slots...)
}

//------------------------------------------------------------------------------

func (c cmdable) GeoAdd(ctx context.Context, key string, geoLocation ...*GeoLocation) *IntCmd {
	args := make([]interface{}, 2+3*len(geoLocation))
	args[0] = "geoadd"
	args[1] = key
	for i, eachLoc := range geoLocation {
		args[2+3*i] = eachLoc.Longitude
		args[2+3*i+1] = eachLoc.Latitude
		args[2+3*i+2] = eachLoc.Name
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// GeoRadius is a read-only GEORADIUS_RO command.
func (c cmdable) GeoRadius(
	ctx context.Context, key string, longitude, latitude float64, query *GeoRadiusQuery,
) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(ctx, query, "georadius_ro", key, longitude, latitude)
	if query.Store != "" || query.StoreDist != "" {
		cmd.SetErr(errors.New("GeoRadius does not support Store or StoreDist"))
		return cmd
	}
	_ = c(ctx, cmd)
	return cmd
}

// GeoRadiusStore is a writing GEORADIUS command.
func (c cmdable) GeoRadiusStore(
	ctx context.Context, key string, longitude, latitude float64, query *GeoRadiusQuery,
) *IntCmd {
	args := geoLocationArgs(query, "georadius", key, longitude, latitude)
	cmd := NewIntCmd(ctx, args...)
	if query.Store == "" && query.StoreDist == "" {
		cmd.SetErr(errors.New("GeoRadiusStore requires Store or StoreDist"))
		return cmd
	}
	_ = c(ctx, cmd)
	return cmd
}

// GeoRadiusByMember is a read-only GEORADIUSBYMEMBER_RO command.
func (c cmdable) GeoRadiusByMember(
	ctx context.Context, key, member string, query *GeoRadiusQuery,
) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(ctx, query, "georadiusbymember_ro", key, member)
	if query.Store != "" || query.StoreDist != "" {
		cmd.SetErr(errors.New("GeoRadiusByMember does not support Store or StoreDist"))
		return cmd
	}
	_ = c(ctx, cmd)
	return cmd
}

// GeoRadiusByMemberStore is a writing GEORADIUSBYMEMBER command.
func (c cmdable) GeoRadiusByMemberStore(
	ctx context.Context, key, member string, query *GeoRadiusQuery,
) *IntCmd {
	args := geoLocationArgs(query, "georadiusbymember", key, member)
	cmd := NewIntCmd(ctx, args...)
	if query.Store == "" && query.StoreDist == "" {
		cmd.SetErr(errors.New("GeoRadiusByMemberStore requires Store or StoreDist"))
		return cmd
	}
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoSearch(ctx context.Context, key string, q *GeoSearchQuery) *StringSliceCmd {
	args := make([]interface{}, 0, 13)
	args = append(args, "geosearch", key)
	args = geoSearchArgs(q, args)
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoSearchLocation(
	ctx context.Context, key string, q *GeoSearchLocationQuery,
) *GeoSearchLocationCmd {
	args := make([]interface{}, 0, 16)
	args = append(args, "geosearch", key)
	args = geoSearchLocationArgs(q, args)
	cmd := NewGeoSearchLocationCmd(ctx, q, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoSearchStore(ctx context.Context, key, store string, q *GeoSearchStoreQuery) *IntCmd {
	args := make([]interface{}, 0, 15)
	args = append(args, "geosearchstore", store, key)
	args = geoSearchArgs(&q.GeoSearchQuery, args)
	if q.StoreDist {
		args = append(args, "storedist")
	}
	cmd := NewIntCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoDist(
	ctx context.Context, key string, member1, member2, unit string,
) *FloatCmd {
	if unit == "" {
		unit = "km"
	}
	cmd := NewFloatCmd(ctx, "geodist", key, member1, member2, unit)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoHash(ctx context.Context, key string, members ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "geohash"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewStringSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) GeoPos(ctx context.Context, key string, members ...string) *GeoPosCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "geopos"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewGeoPosCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ACLDryRun(ctx context.Context, username string, command ...interface{}) *StringCmd {
	args := make([]interface{}, 0, 3+len(command))
	args = append(args, "acl", "dryrun", username)
	args = append(args, command...)
	cmd := NewStringCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// ModuleLoadexConfig struct is used to specify the arguments for the MODULE LOADEX command of redis.
// `MODULE LOADEX path [CONFIG name value [CONFIG name value ...]] [ARGS args [args ...]]`
type ModuleLoadexConfig struct {
	Path string
	Conf map[string]interface{}
	Args []interface{}
}

func (c *ModuleLoadexConfig) toArgs() []interface{} {
	args := make([]interface{}, 3, 3+len(c.Conf)*3+len(c.Args)*2)
	args[0] = "MODULE"
	args[1] = "LOADEX"
	args[2] = c.Path
	for k, v := range c.Conf {
		args = append(args, "CONFIG", k, v)
	}
	for _, arg := range c.Args {
		args = append(args, "ARGS", arg)
	}
	return args
}

// ModuleLoadex Redis `MODULE LOADEX path [CONFIG name value [CONFIG name value ...]] [ARGS args [args ...]]` command.
func (c cmdable) ModuleLoadex(ctx context.Context, conf *ModuleLoadexConfig) *StringCmd {
	cmd := NewStringCmd(ctx, conf.toArgs()...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ACLLog(ctx context.Context, count int64) *ACLLogCmd {
	args := make([]interface{}, 0, 3)
	args = append(args, "acl", "log")
	if count > 0 {
		args = append(args, count)
	}
	cmd := NewACLLogCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ACLLogReset(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "acl", "log", "reset")
	_ = c(ctx, cmd)
	return cmd
}
