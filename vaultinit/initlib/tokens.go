package initlib

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vaulthelper/system"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

//UploadTokens accepts a file directory and vault object to upload tokens to. Logs to pased logger
func UploadTokens(dir string, v *sys.Vault, logger *log.Logger) []*pb.InitResp_Token {
	tokens := []*pb.InitResp_Token{}
	logger.SetPrefix("[TOKEN]")
	logger.Printf("Writing tokens from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger, true)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".yml" || ext == ".yaml" { // Request token from vault
			logger.Printf("\tFound token file: %s\n", file.Name())
			tokenName, err := v.CreateTokenFromFile(dir + "/" + file.Name())
			utils.LogErrorObject(err, logger, true)

			if err == nil {
				fmt.Printf("Created token %-30s %s\n", filename+":", tokenName)
				tokens = append(tokens, &pb.InitResp_Token{
					Name:  filename,
					Value: tokenName,
				})
			}
		}

	}
	return tokens
}