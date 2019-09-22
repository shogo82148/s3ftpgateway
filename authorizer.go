package main

import (
	"context"
	"errors"
	"sort"

	"github.com/shogo82148/s3ftpgateway/ftp"
	"golang.org/x/crypto/bcrypt"
)

// NewAuthorizer returns new authorizer.
func NewAuthorizer(config AuthorizerConfig) (ftp.Authorizer, error) {
	switch config.Method {
	case "userlist":
		return newUserListAuthorizer(config.Config)
	}
	return nil, errors.New("known authorize method")
}

func newUserListAuthorizer(config map[string]interface{}) (ftp.Authorizer, error) {
	users, ok := config["users"].([]interface{})
	if !ok {
		return nil, errors.New("users must be an array")
	}

	list := make(authUsers, 0, len(users))
	for _, user := range users {
		u, ok := user.(map[interface{}]interface{})
		if !ok {
			return nil, errors.New("user must be a map")
		}
		name, ok := u["name"].(string)
		if !ok {
			return nil, errors.New("name must be a string")
		}
		password, ok := u["password"].(string)
		if !ok {
			return nil, errors.New("password must be a string")
		}
		list = append(list, &authUser{
			Name:     name,
			Password: password,
		})
	}
	sort.Sort(list) // TODO: check duplicated user name.
	return userListAuthorizer{
		Users: list,
	}, nil
}

type authUser struct {
	Name     string
	Password string
}

type authUsers []*authUser

func (users authUsers) Len() int           { return len(users) }
func (users authUsers) Less(i, j int) bool { return users[i].Name < users[j].Name }
func (users authUsers) Swap(i, j int)      { users[i], users[j] = users[j], users[i] }

type userListAuthorizer struct {
	// Users is a list of users who can access ftp.
	// It is sorted by the user's name.
	Users authUsers
}

func (a userListAuthorizer) Authorize(ctx context.Context, conn *ftp.ServerConn, user, password string) (*ftp.Authorization, error) {
	found := sort.Search(len(a.Users), func(i int) bool {
		return a.Users[i].Name >= user
	})
	if found >= len(a.Users) {
		return nil, ftp.ErrAuthorizeFailed
	}

	u := a.Users[found]
	if u.Name != user {
		return nil, ftp.ErrAuthorizeFailed
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ftp.ErrAuthorizeFailed
	}
	return &ftp.Authorization{
		User:       user,
		FileSystem: conn.Server().FileSystem,
	}, nil
}
