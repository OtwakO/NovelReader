package config

import "github.com/otwako/novelreader/internal/chapterresource"

func chapterResourceLimits() chapterresource.Limits {
	limits := chapterresource.DefaultLimits()
	limits.TotalBytes = resourceBytes("CHAPTER_RESOURCE_CACHE_MAX_MIB", limits.TotalBytes)
	limits.ReaderBytes = resourceBytes("CHAPTER_RESOURCE_CACHE_READER_MAX_MIB", limits.ReaderBytes)
	limits.TotalBundles = getEnvPositiveInt("CHAPTER_RESOURCE_CACHE_MAX_BUNDLES", limits.TotalBundles)
	limits.ReaderBundles = getEnvPositiveInt("CHAPTER_RESOURCE_CACHE_READER_MAX_BUNDLES", limits.ReaderBundles)
	return limits
}

// Follow existing positive-env fallback semantics, including overflow. Reader
// limits may exceed total limits: admission always enforces both independently.
func resourceBytes(key string, fallback int64) int64 {
	value := int64(getEnvPositiveInt(key, int(fallback>>20)))
	if value > (1<<63-1)>>20 {
		return fallback
	}
	return value << 20
}
