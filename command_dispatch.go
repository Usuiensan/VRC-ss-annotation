package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

func runSubcommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "watch":
		fs := flag.NewFlagSet("watch", flag.ExitOnError)
		root := fs.String("root", appConfig.Watcher.VRChatPhotoRoot, "VRChat写真フォルダ")
		amazonDir := fs.String("amazon-output-dir", appConfig.Watcher.AmazonPhotosOutputDir, "Amazon Photos用出力ディレクトリ")
		_ = fs.Parse(args[1:])
		if *root != "" {
			appConfig.Watcher.VRChatPhotoRoot = *root
		}
		if *amazonDir != "" {
			appConfig.Watcher.AmazonPhotosOutputDir = *amazonDir
		}
		return true, watchPhotoRoot()
	case "process-file":
		fs := flag.NewFlagSet("process-file", flag.ExitOnError)
		amazonDir := fs.String("amazon-output-dir", appConfig.Watcher.AmazonPhotosOutputDir, "Amazon Photos用出力ディレクトリ")
		_ = fs.Parse(args[1:])
		if *amazonDir != "" {
			appConfig.Watcher.AmazonPhotosOutputDir = *amazonDir
		}
		if fs.NArg() != 1 {
			return true, errors.New("process-file には画像パスを1つだけ指定してください")
		}
		entry := processWatchedFile(fs.Arg(0), true)
		if entry.Error != "" {
			return true, errors.New(entry.Error)
		}
		return true, nil
	case "test-eagle":
		return true, testEagleConnection()
	case "eagle-index":
		if len(args) != 2 {
			return true, errors.New("eagle-index には build / sync / status を指定してください")
		}
		idx, err := getEagleHashIndex()
		if err != nil {
			return true, err
		}
		switch args[1] {
		case "build":
			err = idx.Sync(true)
		case "sync":
			err = idx.Sync(false)
		case "status":
			library, count, statusErr := idx.Status()
			if statusErr != nil {
				return true, statusErr
			}
			fmt.Printf("Eagleライブラリ: %s\n登録済みハッシュ: %d 件\n", library, count)
			return true, nil
		default:
			return true, errors.New("eagle-index には build / sync / status を指定してください")
		}
		if err == nil {
			fmt.Println("Eagleハッシュ索引を同期しました")
		}
		return true, err
	case "print-config":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return true, enc.Encode(appConfig)
	case "retry-failed":
		return true, retryFailed()
	case "reprocess-state":
		return true, reprocessState()
	default:
		return false, nil
	}
}
