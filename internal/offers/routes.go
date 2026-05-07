package offers

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *OfferHandler, authMiddleware gin.HandlerFunc) {
	public := r.Group("/api/v1")
	{
		public.GET("/offers", h.GetPublicOffers)
		public.GET("/offers/:id", h.GetOfferByID)
	}

	partner := r.Group("/api/v1/partner")
	partner.Use(authMiddleware)
	partner.Use(func(c *gin.Context) {
		role := c.GetString("role")
		if role != "PARTNER" {
			c.AbortWithStatusJSON(403, gin.H{
				"error":   "FORBIDDEN",
				"message": "Only partners can manage offers",
			})
			return
		}
		c.Next()
	})
	{
		partner.GET("/offers", h.GetPartnerOffers)
		partner.POST("/offers", h.CreateOffer)
		partner.POST("/offers/upload", h.UploadImage)
		partner.PATCH("/offers/:id", h.UpdateOffer)
		partner.DELETE("/offers/:id", h.DeleteOffer)
	}
}
