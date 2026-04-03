package controller

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"zdir/config"

	"github.com/gin-gonic/gin"
)

// 定义一个结构体，用来存放文件或文件夹信息
type info struct {
	Name  string
	Size  int64
	Mtime string
	Ftype string
	Fpath string
	Ext   string
	Link  string
}

// 预编译隐藏文件正则，避免每次请求重新编译
var hiddenFileRe = regexp.MustCompile(`^(\.|@|#).*`)

// GetDirCacheTTL 返回目录列表缓存时间（秒），0表示不缓存
// 网络挂载盘建议设置较大的值（如60秒），本地磁盘可设为0
func GetDirCacheTTL() int {
	return config.DirCacheTTL()
}

// 获取文件列表
func FileList(c *gin.Context) {
	//获取公共存储的路径
	public_dir := config.Public_path()
	storage_domain := config.Public_domain(c)
	//获取请求参数
	path := c.Query("path")
	//判断用户传递的路径是否合法
	if !V_fpath(path) {
		c.JSON(200, gin.H{
			"code": -1000,
			"msg":  "文件夹不合法！",
			"data": "",
		})
		c.Abort()
		return
	}

	//组合完整路径
	var full_path string
	if path == "" {
		full_path = public_dir
	} else {
		full_path = public_dir + "/" + path
	}

	//判断文件是否存在，如果不存在，则终止执行
	_, err := os.Stat(full_path)
	if os.IsNotExist(err) {
		c.JSON(200, gin.H{
			"code": -1000,
			"msg":  "文件夹不存在！",
			"data": "",
		})
		c.Abort()
		return
	}

	// 检查目录列表缓存
	cacheTTL := GetDirCacheTTL()
	cacheKey := []byte("dirlist:" + full_path)
	if cacheTTL > 0 {
		if cached := GetCache(cacheKey); len(cached) > 0 {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Header("X-Cache", "HIT")
			c.String(200, string(cached))
			return
		}
	}

	// 使用 os.ReadDir 替代 ioutil.ReadDir：
	// 1. ioutil.ReadDir 已废弃
	// 2. os.ReadDir 返回 DirEntry，获取基础信息无需额外 stat 调用
	entries, err := os.ReadDir(full_path)
	if err != nil {
		log.Print(err)
		c.JSON(200, gin.H{
			"code": 500,
			"msg":  "读取目录失败！",
			"data": "",
		})
		return
	}

	sort_result := []info{}
	sort_result_file := []info{}

	for _, entry := range entries {
		fname := entry.Name()

		// 隐藏文件过滤（使用预编译正则）
		if hiddenFileRe.MatchString(fname) {
			continue
		}

		var new_info info
		var ftype string

		if entry.IsDir() {
			ftype = "folder"
			new_info.Ext = ""
			new_info.Link = ""
		} else {
			ftype = "file"
			// 安全地获取文件扩展名，避免空文件名或没有后缀的情况
			if idx := strings.LastIndexByte(fname, '.'); idx > 0 && idx < len(fname)-1 {
				new_info.Ext = strings.ToLower(fname[idx+1:])
			} else {
				new_info.Ext = ""
			}
		}

		// 获取文件元信息：只有在需要 Size/Mtime 时才调用 Info()
		// 对于网络挂载盘，DirEntry.Info() 通常可直接从目录条目获取，避免额外 stat
		fi, err := entry.Info()
		if err != nil {
			log.Print(err)
			continue
		}

		new_info.Ftype = ftype
		new_info.Mtime = fi.ModTime().Format("2006-01-02 15:04:05")
		new_info.Size = fi.Size()
		new_info.Name = fname
		new_info.Fpath = path + "/" + fname
		new_info.Link = storage_domain + url.QueryEscape(new_info.Fpath)

		if ftype == "folder" {
			sort_result = append(sort_result, new_info)
		} else {
			sort_result_file = append(sort_result_file, new_info)
		}
	}

	// 文件夹在前，文件在后
	sort_result = append(sort_result, sort_result_file...)

	resp := gin.H{
		"code": 200,
		"msg":  "success",
		"data": sort_result,
	}

	// 如果开启了目录缓存，将结果序列化后存入缓存
	if cacheTTL > 0 {
		if jsonBytes, err := json.Marshal(resp); err == nil {
			SetCache(cacheKey, jsonBytes, cacheTTL)
		}
	}

	c.JSON(200, resp)
}
