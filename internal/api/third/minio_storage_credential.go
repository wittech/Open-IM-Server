package apiThird

import (
	apiStruct "Open_IM/pkg/base_info"
	"Open_IM/pkg/common/config"
	"Open_IM/pkg/common/constant"
	imdb "Open_IM/pkg/common/db/mysql_model/im_mysql_model"
	"Open_IM/pkg/common/log"
	"Open_IM/pkg/common/token_verify"
	_ "Open_IM/pkg/common/token_verify"
	"Open_IM/pkg/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	_ "github.com/minio/minio-go/v7"
	cr "github.com/minio/minio-go/v7/pkg/credentials"
	"net/http"
	"path"
)

func MinioUploadFile(c *gin.Context) {
	var (
		req  apiStruct.MinioUploadFileReq
		resp apiStruct.MinioUploadFileResp
	)
	defer func() {
		if r := recover(); r != nil {
			log.NewError(req.OperationID, utils.GetSelfFuncName(), r)
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file or snapShot args"})
			return
		}
	}()
	if err := c.Bind(&req); err != nil {
		log.NewError("0", utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	ok, _ := token_verify.GetUserIDFromToken(c.Request.Header.Get("token"), req.OperationID)
	if !ok {
		log.NewError("", utils.GetSelfFuncName(), "GetUserIDFromToken false ", c.Request.Header.Get("token"))
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "GetUserIDFromToken failed"})
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), req)
	switch req.FileType {
	// videoType upload snapShot
	case constant.VideoType:
		snapShotFile, err := c.FormFile("snapShot")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing snapshot arg: " + err.Error()})
			return
		}
		snapShotFileObj, err := snapShotFile.Open()
		if err != nil {
			log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
			return
		}
		snapShotNewName, snapShotNewType := utils.GetNewFileNameAndContentType(snapShotFile.Filename, constant.ImageType)
		log.Debug(req.OperationID, utils.GetSelfFuncName(), snapShotNewName, snapShotNewType)
		_, err = minioClient.PutObject(context.Background(), config.Config.Credential.Minio.Bucket, snapShotNewName, snapShotFileObj, snapShotFile.Size, minio.PutObjectOptions{ContentType: snapShotNewType})
		if err != nil {
			log.NewError(req.OperationID, utils.GetSelfFuncName(), "PutObject snapShotFile error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
			return
		}
		resp.SnapshotURL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.Bucket + "/" + snapShotNewName
		resp.SnapshotNewName = snapShotNewName
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file arg: " + err.Error()})
		return
	}
	fileObj, err := file.Open()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file path" + err.Error()})
		return
	}
	newName, newType := utils.GetNewFileNameAndContentType(file.Filename, req.FileType)
	log.Debug(req.OperationID, utils.GetSelfFuncName(), newName, newType)
	_, err = minioClient.PutObject(context.Background(), config.Config.Credential.Minio.Bucket, newName, fileObj, file.Size, minio.PutObjectOptions{ContentType: newType})
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "open file error")
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "invalid file path" + err.Error()})
		return
	}
	resp.NewName = newName
	resp.URL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.Bucket + "/" + newName
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "resp: ", resp)
	c.JSON(http.StatusOK, gin.H{"errCode": 0, "errMsg": "", "data": resp})
	return
}

func MinioStorageCredential(c *gin.Context) {
	var (
		req  apiStruct.MinioStorageCredentialReq
		resp apiStruct.MiniostorageCredentialResp
	)
	if err := c.BindJSON(&req); err != nil {
		log.NewError("0", utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	ok, _ := token_verify.GetUserIDFromToken(c.Request.Header.Get("token"), req.OperationID)
	if !ok {
		log.NewError("", utils.GetSelfFuncName(), "GetUserIDFromToken false ", c.Request.Header.Get("token"))
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "GetUserIDFromToken failed"})
		return
	}
	var stsOpts cr.STSAssumeRoleOptions
	stsOpts.AccessKey = config.Config.Credential.Minio.AccessKeyID
	stsOpts.SecretKey = config.Config.Credential.Minio.SecretAccessKey
	stsOpts.DurationSeconds = constant.MinioDurationTimes
	var endpoint string
	if config.Config.Credential.Minio.EndpointInnerEnable {
		endpoint = config.Config.Credential.Minio.EndpointInner
	} else {
		endpoint = config.Config.Credential.Minio.Endpoint
	}
	li, err := cr.NewSTSAssumeRole(endpoint, stsOpts)
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "NewSTSAssumeRole failed", err.Error(), stsOpts, config.Config.Credential.Minio.Endpoint)
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	v, err := li.Get()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "li.Get error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	resp.SessionToken = v.SessionToken
	resp.SecretAccessKey = v.SecretAccessKey
	resp.AccessKeyID = v.AccessKeyID
	resp.BucketName = config.Config.Credential.Minio.Bucket
	resp.StsEndpointURL = config.Config.Credential.Minio.Endpoint
	c.JSON(http.StatusOK, gin.H{"errCode": 0, "errMsg": "", "data": resp})
}

func UploadUpdateApp(c *gin.Context) {
	var (
		req  apiStruct.UploadUpdateAppReq
		resp apiStruct.UploadUpdateAppResp
	)
	if err := c.Bind(&req); err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "req: ", req)

	fileObj, err := req.File.Open()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "Open file error" + err.Error()})
		return
	}
	yamlObj, err := req.Yaml.Open()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open Yaml error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "Open Yaml error" + err.Error()})
		return
	}

	// v2.0.9_app_linux v2.0.9_yaml_linux

	//file, err := c.FormFile("file")
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "FormFile failed", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file arg: " + err.Error()})
	//	return
	//}
	//fileObj, err := file.Open()
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file path" + err.Error()})
	//	return
	//}
	//
	//yaml, err := c.FormFile("yaml")
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "FormFile failed", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file arg: " + err.Error()})
	//	return
	//}
	//yamlObj, err := yaml.Open()
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file path" + err.Error()})
	//	return
	//}
	newFileName, newYamlName, err := utils.GetUploadAppNewName(req.Type, req.Version, req.File.Filename, req.Yaml.Filename)
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "GetUploadAppNewName failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file type" + err.Error()})
		return
	}
	fmt.Println(req.OperationID, utils.GetSelfFuncName(), "name: ", config.Config.Credential.Minio.AppBucket, newFileName, fileObj, req.File.Size)
	fmt.Println(req.OperationID, utils.GetSelfFuncName(), "name: ", config.Config.Credential.Minio.AppBucket, newYamlName, yamlObj, req.Yaml.Size)

	_, err = MinioClient.PutObject(context.Background(), config.Config.Credential.Minio.AppBucket, newFileName, fileObj, req.File.Size, minio.PutObjectOptions{ContentType: path.Ext(newFileName)})
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "PutObject file error")
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "PutObject file error" + err.Error()})
		return
	}
	_, err = MinioClient.PutObject(context.Background(), config.Config.Credential.Minio.AppBucket, newYamlName, yamlObj, req.Yaml.Size, minio.PutObjectOptions{ContentType: path.Ext(newYamlName)})
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "PutObject yaml error")
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "PutObject yaml error" + err.Error()})
		return
	}
	if err := imdb.UpdateAppVersion(req.Type, req.Version, req.ForceUpdate, newFileName, newYamlName); err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "UpdateAppVersion error", err.Error())
		resp.ErrCode = http.StatusInternalServerError
		resp.ErrMsg = err.Error()
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName())
	c.JSON(http.StatusOK, resp)
}

func GetDownloadURL(c *gin.Context) {
	var (
		req  apiStruct.GetDownloadURLReq
		resp apiStruct.GetDownloadURLResp
	)
	defer func() {
		log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "resp: ", resp)
	}()
	if err := c.Bind(&req); err != nil {
		log.NewError("0", utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "req: ", req)
	//fileName, yamlName, err := utils.GetUploadAppNewName(req.Type, req.Version, req.)
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "GetUploadAppNewName failed", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file type" + err.Error()})
	//	return
	//}
	app, err := imdb.GetNewestVersion(req.Type)
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "getNewestVersion failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "getNewestVersion failed" + err.Error()})
		return
	}
	if app.Version != req.Version {
		resp.Data.HasNewVersion = true
		if app.ForceUpdate == true {
			resp.Data.ForceUpdate = true
		}
		resp.Data.YamlURL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.AppBucket + "/" + app.YamlName
		resp.Data.FileURL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.AppBucket + "/" + app.FileName
		c.JSON(http.StatusOK, resp)
	} else {
		resp.Data.HasNewVersion = false
		c.JSON(http.StatusOK, resp)
	}
}
