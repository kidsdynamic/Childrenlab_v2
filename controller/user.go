package controller

import (
	"net/http"

	"log"

	"database/sql"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/kidsdynamic/childrenlab_v2/database"
	"github.com/kidsdynamic/childrenlab_v2/model"
)

func Login(c *gin.Context) {
	var json model.Login

	if c.BindJSON(&json) != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	log.Printf("\nEmail: %s, Password:%s\n", json.Email, json.Password)
	db := database.New()
	defer db.Close()
	var user model.User

	json.Password = EncryptPassword(json.Password)

	err := db.Get(&user,
		"SELECT email, COALESCE(first_name, '') as first_name, COALESCE(last_name, '') as last_name, "+
			" COALESCE(zip_code, '') as zip_code, last_updated, date_created FROM user WHERE email=? and password=? LIMIT 1",
		json.Email,
		json.Password)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	log.Printf("\nUser login request. User: %#v\n", user)

	accessToken := model.AccessToken{
		Email: user.Email,
		Token: randToken(),
	}

	success := storeToken(db, accessToken)

	if !success {
		log.Println("Store token fail!!!!")
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Store token failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     accessToken.Email,
		"access_token": accessToken.Token,
	})

}

func storeToken(db *sqlx.DB, accessToken model.AccessToken) bool {
	var existToken model.AccessToken
	err := db.Get(&existToken, "SELECT email, token, last_updated FROM authentication_token WHERE email = ?", accessToken.Email)

	var result sql.Result
	if err != nil {
		result = db.MustExec("INSERT INTO authentication_token (email, token, date_created, last_updated) VALUES (?,?, Now(), Now())",
			accessToken.Email,
			accessToken.Token)
	} else {
		result = db.MustExec("UPDATE authentication_token SET token = ?, last_updated = NOW(), access_count = access_count + 1 WHERE email = ?",
			accessToken.Token,
			accessToken.Email)

	}

	return checkInsertResult(result)

}

func Register(c *gin.Context) {
	var registerRequest model.Register
	if err := c.BindJSON(&registerRequest); err != nil {
		log.Printf("Register Error: %#v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Missing some of required paramters.",
			"error":   err,
		})
		return
	}

	db := database.New()
	defer db.Close()

	var exist bool
	if err := db.Get(&exist, "SELECT EXISTS(SELECT id FROM user WHERE email = ? LIMIT 1)", registerRequest.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Something wrong on server side",
			"error":   err,
		})
		return
	}

	if exist {
		c.JSON(http.StatusConflict, gin.H{
			"message": "The email is already registered",
		})
		return
	}

	registerRequest.Password = EncryptPassword(registerRequest.Password)

	result, err := db.NamedExec("INSERT INTO user (email, password, first_name, last_name, phone_number, zip_code, date_created, last_updated) VALUES"+
		" (:email, :password, :first_name, :last_name, :phone_number, :zip_code, Now(), Now())",
		registerRequest)

	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Error when insert User to database",
			"error":   err,
		})
		return
	}

	if checkInsertResult(result) {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "The data was not able to write to database. No error",
		})
	}

}

func IsTokenValid(c *gin.Context) {
	var tokenRequest model.TokenRequest

	if c.BindJSON(&tokenRequest) != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	var existToken model.AccessToken
	db := database.New()
	defer db.Close()
	err := db.Get(&existToken, "SELECT email, token, last_updated FROM authentication_token WHERE email = ? AND token = ?",
		tokenRequest.Email,
		tokenRequest.Token)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{})

}

func UpdateProfile(c *gin.Context) {
	var request model.ProfileUpdateRequest

	if err := c.BindJSON(&request); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	fmt.Printf("Profile Update Request: %#v", request)

	signedInUser := GetSignedInUser(c)

	db := database.New()
	defer db.Close()

	tx := db.MustBegin()
	if request.FirstName != "" {
		tx.MustExec("UPDATE user SET first_name = ? WHERE id = ?", request.FirstName, signedInUser.ID)
	}

	if request.LastName != "" {
		tx.MustExec("UPDATE user SET last_name = ? WHERE id = ?", request.LastName, signedInUser.ID)
	}

	if request.PhoneNumber != "" {
		tx.MustExec("UPDATE user SET phone_number = ? WHERE id = ?", request.PhoneNumber, signedInUser.ID)
	}

	if request.ZipCode != "" {
		tx.MustExec("UPDATE user SET zip_code = ? WHERE id = ?", request.ZipCode, signedInUser.ID)
	}

	tx.MustExec("UPDATE user SET last_updated = NOW() WHERE id = ?", signedInUser.ID)
	tx.Commit()

	user, err := GetUserByID(db, signedInUser.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Something wrong when retreive updated user information",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func UserProfile(c *gin.Context) {
	user := GetSignedInUser(c)

	kids, err := GetKidsByUser(user)
	if err != nil {
		fmt.Printf("Kids error: %#v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error when retrieve kids",
			"error":   err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
		"kids": kids,
	})

}

func IsEmailAvailableToRegister(c *gin.Context) {
	email := c.Query("email")

	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	db := database.New()
	defer db.Close()

	var exist bool
	if err := db.Get(&exist, "SELECT EXISTS(SELECT id FROM user WHERE email = ? LIMIT 1)", email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Something wrong on server side",
			"error":   err,
		})
		return
	}

	if exist {
		c.JSON(http.StatusConflict, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

type iOS struct {
	RegistrationId string
}

func UpdateIOSRegistrationId(c *gin.Context) {
	var ios iOS

	err := c.BindJSON(&ios)

	if err != nil {
		log.Printf("Error on UpdateIosRegistrationId: Error: %#v", err)
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	db := database.New()
	defer db.Close()

	user := GetSignedInUser(c)

	if _, err := db.Exec("UPDATE user SET registration_id = ? WHERE id = ?", ios.RegistrationId, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Something wrong on server side",
			"error":   err,
		})
		return
	}

	updatedUser, _ := GetUserByID(db, user.ID)

	c.JSON(http.StatusOK, updatedUser)
}
