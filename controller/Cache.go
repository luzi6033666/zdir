package controller

import (
	"fmt"

	"github.com/coocood/freecache"
	"github.com/gin-gonic/gin"
)

// 声明全局缓存参数
var Cache *freecache.Cache

// 缓存初始化
func init() {
	//设置一个最大为100M的缓存
	cacheSize := 100 * 1024 * 1024
	//初始化缓存
	Cache = freecache.NewCache(cacheSize)
}

// 缓存设置
func SetCache(key []byte, value []byte, ttl int) {
	//设置缓存
	Cache.Set(key, value, ttl)
}

// 获取缓存
func GetCache(key []byte) []byte {
	got, err := Cache.Get(key)
	if err != nil {
		empty := []byte("")
		return empty
	} else {
		return got
	}
}

// 删除缓存
func DelCache(key string) bool {
	new_key := []byte(key)
	_ = Cache.Del(new_key)
	return true
}

// ClearDirCache 清除所有目录列表缓存（管理员 API）
// 适用于网络挂载盘在 TTL 未到期时手动刷新
func ClearDirCache(c *gin.Context) {
	before := Cache.EntryCount()
	Cache.Clear()
	fmt.Printf("cache cleared, removed %d entries\n", before)
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "缓存已清除",
		"data": gin.H{"cleared_entries": before},
	})
}
