package httpx

import "github.com/labstack/echo/v5"

const authUserKey = "auth.userId"

func SetAuthUser(c *echo.Context, userID string) {
	c.Set(authUserKey, userID)
}

func AuthUserID(c *echo.Context) string {
	userID, _ := c.Get(authUserKey).(string)
	return userID
}
