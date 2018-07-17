package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
	gql "github.com/graphql-go/graphql"
)

//Server implements the twirp api server endpoints
type Server struct {
	VaultToken string
	VaultAddr  string
	CertPath   string
	GQLSchema  gql.Schema
	Log        *log.Logger
}

//NewServer Creates a new server struct and initializes the GraphQL schema
func NewServer(VaultAddr string, VaultToken string, CertPath string) *Server {
	s := Server{}
	s.VaultToken = VaultToken
	s.VaultAddr = VaultAddr
	s.CertPath = CertPath
	s.Log = log.New(os.Stdout, "[INFO]", log.LstdFlags)

	return &s
}

//ListServiceTemplates lists the templates under the requested service
func (s *Server) ListServiceTemplates(ctx context.Context, req *pb.ListReq) (*pb.ListResp, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	listPath := "templates/" + req.Service
	secret, err := mod.List(listPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	if secret == nil {
		err := fmt.Errorf("Could not find any templates under %s", req.Service)
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	utils.LogWarningsObject(secret.Warnings, s.Log, false)
	if len(secret.Warnings) > 0 {
		err := errors.New("Warnings generated from vault " + req.Service)
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	fileNames := []string{}
	for _, fileName := range secret.Data["keys"].([]interface{}) {
		if strFile, ok := fileName.(string); ok {
			if strFile[len(strFile)-1] != '/' { // Skip subdirectories where template files are stored
				fileNames = append(fileNames, strFile)
			}
		}
	}

	return &pb.ListResp{
		Templates: fileNames,
	}, nil
}

// GetTemplate makes a request to the vault for the template found in <service>/<file>/template-file
// Returns the template data in base64 and the template's extension. Returns any errors generated by vault
func (s *Server) GetTemplate(ctx context.Context, req *pb.TemplateReq) (*pb.TemplateResp, error) {
	// Connect to the vault
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	// Get template data from information in request.
	path := "templates/" + req.Service + "/" + req.File + "/template-file"
	data, err := mod.ReadData(path)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	if data == nil {
		err := errors.New("No file " + req.File + " under " + req.Service)
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	// Return retrieved data in response
	return &pb.TemplateResp{
		Data: data["data"].(string),
		Ext:  data["ext"].(string)}, nil
}

// Validate checks the vault to see if the requested credentials are validated
func (s *Server) Validate(ctx context.Context, req *pb.ValidationReq) (*pb.ValidationResp, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}
	mod.Env = req.Env

	servicePath := "verification/" + req.Service
	data, err := mod.ReadData(servicePath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	if data == nil {
		err := errors.New("No verification for " + req.Service + " found under " + req.Env + " environment")
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	return &pb.ValidationResp{IsValid: data["verified"].(bool)}, nil
}

//GetValues gets values requested from the vault
func (s *Server) GetValues(ctx context.Context, req *pb.GetValuesReq) (*pb.ValuesRes, error) {

	environments := []*pb.ValuesRes_Env{}
	envStrings := []string{"dev", "QA", "local"}
	for _, environment := range envStrings {
		mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}
		mod.Env = environment
		services := []*pb.ValuesRes_Env_Service{}
		//get a list of services under values
		servicePaths, err := s.getPaths(mod, "values/")
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}

		for _, servicePath := range servicePaths {
			files := []*pb.ValuesRes_Env_Service_File{}
			//get a list of files under service
			filePaths, err := s.getPaths(mod, servicePath)
			if err != nil {
				utils.LogErrorObject(err, s.Log, false)
				return nil, err
			}

			for _, filePath := range filePaths {
				vals := []*pb.ValuesRes_Env_Service_File_Value{}
				//get a list of values
				valueMap, err := mod.ReadData(filePath)
				if err != nil {
					err := fmt.Errorf("Unable to fetch data from %s", filePath)
					utils.LogErrorObject(err, s.Log, false)
					return nil, err
				}
				if valueMap != nil {
					//fmt.Println("data at path " + path)
					for key, value := range valueMap {
						kv := &pb.ValuesRes_Env_Service_File_Value{Key: key, Value: value.(string)}
						vals = append(vals, kv)
						//data = append(data, value.(string))
					}

				}
				file := &pb.ValuesRes_Env_Service_File{Name: getPathEnd(filePath), Values: vals}
				files = append(files, file)
			}
			service := &pb.ValuesRes_Env_Service{Name: getPathEnd(servicePath), Files: files}
			services = append(services, service)
		}
		env := &pb.ValuesRes_Env{Name: environment, Services: services}
		environments = append(environments, env)
	}
	return &pb.ValuesRes{
		Envs: environments,
	}, nil
}
func (s *Server) getPaths(mod *kv.Modifier, pathName string) ([]string, error) {
	secrets, err := mod.List(pathName)
	pathList := []string{}
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, fmt.Errorf("Unable to list paths under %s in %s", pathName, mod.Env)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		//fmt.Println("secrets are")
		//fmt.Println(slicey)
		for _, pathEnd := range slicey {
			//List is returning both pathEnd and pathEnd/
			path := pathName + pathEnd.(string)
			pathList = append(pathList, path)
		}
		return pathList, nil
	}
	return pathList, nil
}
func getPathEnd(path string) string {
	strs := strings.Split(path, "/")
	for strs[len(strs)-1] == "" {
		strs = strs[:len(strs)-1]
	}
	return strs[len(strs)-1]
}
func (s *Server) UpdateAPI(ctx context.Context, req *pb.UpdateAPIReq) (*pb.UpdateAPIResp, error) {
	if len(req.Urls) == 2 {
		apiRouterURL := req.Urls[0]
		vaultUIURL := req.Urls[1]
		err := DownloadFile("/etc/opt/vaultAPI/apiRouter", apiRouterURL)
		if err != nil {
			return nil, err
		}
		err = DownloadFile("/etc/opt/vaultAPI/public.zip", vaultUIURL)
		if err != nil {
			return nil, err
		}
		err = Unzip("/etc/opt/vaultAPI/public.zip", "/etc/opt/vaultAPI/public")

		return nil, nil
		return &pb.UpdateAPIResp{}, nil
	}
	return nil, errors.New("Invalid request")
}
func DownloadFile(filepath string, url string) error {
	//remove the old file
	err := os.RemoveAll(filepath)
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
func Unzip(src string, dest string) error {
	err := os.RemoveAll(dest)
	body, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}
	for _, zipfile := range r.File {
		dst, err := os.Create(zipfile.Name)
		if err != nil {
			return err
		}
		defer dst.Close()
		src, err := zipfile.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		io.Copy(dst, src)
	}
	return nil
}
