package scaleway

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func resourceScalewayLbCertificate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceScalewayLbCertificateCreate,
		ReadContext:   resourceScalewayLbCertificateRead,
		UpdateContext: resourceScalewayLbCertificateUpdate,
		DeleteContext: resourceScalewayLbCertificateDelete,
		SchemaVersion: 1,
		Timeouts: &schema.ResourceTimeout{
			Default: schema.DefaultTimeout(defaultLbLbTimeout),
		},
		StateUpgraders: []schema.StateUpgrader{
			{Version: 0, Type: lbUpgradeV1SchemaType(), Upgrade: lbUpgradeV1SchemaUpgradeFunc},
		},
		Schema: map[string]*schema.Schema{
			"lb_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The load-balancer ID",
			},
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the load-balancer certificate",
				Optional:    true,
				Computed:    true,
			},
			"letsencrypt": {
				ConflictsWith: []string{"custom_certificate"},
				MaxItems:      1,
				Description:   "The Let's Encrypt type certificate configuration",
				Type:          schema.TypeList,
				Optional:      true,
				ForceNew:      true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"common_name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The main domain name of the certificate",
						},
						"subject_alternative_name": {
							Type: schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Optional:    true,
							Description: "The alternative domain names of the certificate",
						},
					},
				},
			},
			"custom_certificate": {
				ConflictsWith: []string{"letsencrypt"},
				MaxItems:      1,
				Type:          schema.TypeList,
				Description:   "The custom type certificate type configuration",
				Optional:      true,
				ForceNew:      true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"certificate_chain": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The full PEM-formatted certificate chain",
						},
					},
				},
			},

			// Readonly attributes
			"common_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The main domain name of the certificate",
			},
			"subject_alternative_name": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The alternative domain names of the certificate",
				Elem: &schema.Schema{
					Type:        schema.TypeString,
					Description: "The domain name",
				},
			},
			"fingerprint": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The identifier (SHA-1) of the certificate",
			},
			"not_valid_before": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The not valid before validity bound timestamp",
			},
			"not_valid_after": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The not valid after validity bound timestamp",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of certificate",
			},
		},
	}
}

func resourceScalewayLbCertificateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	zone, lbID, err := parseZonedID(d.Get("lb_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	createReq := &lb.ZonedAPICreateCertificateRequest{
		Zone:              zone,
		LBID:              lbID,
		Name:              expandOrGenerateString(d.Get("name"), "lb-cert"),
		Letsencrypt:       expandLbLetsEncrypt(d.Get("letsencrypt")),
		CustomCertificate: expandLbCustomCertificate(d.Get("custom_certificate")),
	}
	if createReq.Letsencrypt == nil && createReq.CustomCertificate == nil {
		return diag.FromErr(errors.New("you need to define either letsencrypt or custom_certificate configuration"))
	}

	lbAPI, _, err := lbAPIWithZone(d, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	res, err := lbAPI.CreateCertificate(createReq, scw.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newZonedIDString(zone, res.ID))

	return resourceScalewayLbCertificateRead(ctx, d, meta)
}

func resourceScalewayLbCertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	lbAPI, zone, ID, err := lbAPIWithZoneAndID(meta, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	res, err := lbAPI.GetCertificate(&lb.ZonedAPIGetCertificateRequest{
		CertificateID: ID,
		Zone:          zone,
	}, scw.WithContext(ctx))

	if err != nil {
		if is404Error(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	_ = d.Set("lb_id", newZonedIDString(zone, res.LB.ID))
	_ = d.Set("name", res.Name)
	_ = d.Set("common_name", res.CommonName)
	_ = d.Set("subject_alternative_name", res.SubjectAlternativeName)
	_ = d.Set("fingerprint", res.Fingerprint)
	_ = d.Set("not_valid_before", flattenTime(res.NotValidBefore))
	_ = d.Set("not_valid_after", flattenTime(res.NotValidAfter))
	_ = d.Set("status", res.Status)
	return nil
}

func resourceScalewayLbCertificateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	lbAPI, zone, ID, err := lbAPIWithZoneAndID(meta, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	req := &lb.ZonedAPIUpdateCertificateRequest{
		CertificateID: ID,
		Zone:          zone,
		Name:          d.Get("name").(string),
	}

	_, err = lbAPI.UpdateCertificate(req, scw.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceScalewayLbCertificateRead(ctx, d, meta)
}

func resourceScalewayLbCertificateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	lbAPI, zone, ID, err := lbAPIWithZoneAndID(meta, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = lbAPI.DeleteCertificate(&lb.ZonedAPIDeleteCertificateRequest{
		Zone:          zone,
		CertificateID: ID,
	}, scw.WithContext(ctx))

	if err != nil && !is404Error(err) {
		return diag.FromErr(err)
	}

	return nil
}
