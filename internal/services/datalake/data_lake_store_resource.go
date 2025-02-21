package datalake

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/datalake/store/mgmt/2016-11-01/account"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/datalake/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/suppress"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func resourceDataLakeStore() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceArmDateLakeStoreCreate,
		Read:   resourceArmDateLakeStoreRead,
		Update: resourceArmDateLakeStoreUpdate,
		Delete: resourceArmDateLakeStoreDelete,

		// TODO: replace this with an importer which validates the ID during import
		Importer: pluginsdk.DefaultImporter(),

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.AccountName(),
			},

			"location": azure.SchemaLocation(),

			"resource_group_name": azure.SchemaResourceGroupName(),

			"tier": {
				Type:             pluginsdk.TypeString,
				Optional:         true,
				Default:          string(account.Consumption),
				DiffSuppressFunc: suppress.CaseDifference,
				ValidateFunc: validation.StringInSlice([]string{
					string(account.Consumption),
					string(account.Commitment1TB),
					string(account.Commitment10TB),
					string(account.Commitment100TB),
					string(account.Commitment500TB),
					string(account.Commitment1PB),
					string(account.Commitment5PB),
				}, true),
			},

			"encryption_state": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				Default:  string(account.Enabled),
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(account.Enabled),
					string(account.Disabled),
				}, true),
				DiffSuppressFunc: suppress.CaseDifference,
			},

			"encryption_type": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(account.ServiceManaged),
				}, true),
				DiffSuppressFunc: suppress.CaseDifference,
			},

			"firewall_state": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				Default:  string(account.FirewallStateEnabled),
				ValidateFunc: validation.StringInSlice([]string{
					string(account.FirewallStateEnabled),
					string(account.FirewallStateDisabled),
				}, true),
				DiffSuppressFunc: suppress.CaseDifference,
			},

			"firewall_allow_azure_ips": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				Default:  string(account.FirewallAllowAzureIpsStateEnabled),
				ValidateFunc: validation.StringInSlice([]string{
					string(account.FirewallAllowAzureIpsStateEnabled),
					string(account.FirewallAllowAzureIpsStateDisabled),
				}, true),
				DiffSuppressFunc: suppress.CaseDifference,
			},

			"endpoint": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"tags": tags.Schema(),
		},
	}
}

func resourceArmDateLakeStoreCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Datalake.StoreAccountsClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, resourceGroup, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("checking for presence of existing Data Lake Store %q (Resource Group %q): %s", name, resourceGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_data_lake_store", *existing.ID)
		}
	}

	location := azure.NormalizeLocation(d.Get("location").(string))
	tier := d.Get("tier").(string)

	encryptionState := account.EncryptionState(d.Get("encryption_state").(string))
	encryptionType := account.EncryptionConfigType(d.Get("encryption_type").(string))
	firewallState := account.FirewallState(d.Get("firewall_state").(string))
	firewallAllowAzureIPs := account.FirewallAllowAzureIpsState(d.Get("firewall_allow_azure_ips").(string))
	t := d.Get("tags").(map[string]interface{})

	log.Printf("[INFO] preparing arguments for Data Lake Store creation %q (Resource Group %q)", name, resourceGroup)

	dateLakeStore := account.CreateDataLakeStoreAccountParameters{
		Location: &location,
		Tags:     tags.Expand(t),
		CreateDataLakeStoreAccountProperties: &account.CreateDataLakeStoreAccountProperties{
			NewTier:               account.TierType(tier),
			FirewallState:         firewallState,
			FirewallAllowAzureIps: firewallAllowAzureIPs,
			EncryptionState:       encryptionState,
			EncryptionConfig: &account.EncryptionConfig{
				Type: encryptionType,
			},
		},
	}

	future, err := client.Create(ctx, resourceGroup, name, dateLakeStore)
	if err != nil {
		return fmt.Errorf("issuing create request for Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("creating Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	read, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		return fmt.Errorf("retrieving Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Data Lake Store %s (resource group %s) ID", name, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmDateLakeStoreRead(d, meta)
}

func resourceArmDateLakeStoreUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Datalake.StoreAccountsClient
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	tier := d.Get("tier").(string)
	firewallState := account.FirewallState(d.Get("firewall_state").(string))
	firewallAllowAzureIPs := account.FirewallAllowAzureIpsState(d.Get("firewall_allow_azure_ips").(string))
	t := d.Get("tags").(map[string]interface{})

	props := account.UpdateDataLakeStoreAccountParameters{
		UpdateDataLakeStoreAccountProperties: &account.UpdateDataLakeStoreAccountProperties{
			NewTier:               account.TierType(tier),
			FirewallState:         firewallState,
			FirewallAllowAzureIps: firewallAllowAzureIPs,
		},
		Tags: tags.Expand(t),
	}

	future, err := client.Update(ctx, resourceGroup, name, props)
	if err != nil {
		return fmt.Errorf("issuing update request for Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for the update of Data Lake Store %q (Resource Group %q) to commplete: %+v", name, resourceGroup, err)
	}

	return resourceArmDateLakeStoreRead(d, meta)
}

func resourceArmDateLakeStoreRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Datalake.StoreAccountsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	name := id.Path["accounts"]

	resp, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[WARN] Data Lake Store Account %q was not found (Resource Group %q)", name, resourceGroup)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("making Read request on Azure Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	d.Set("name", name)
	d.Set("resource_group_name", resourceGroup)
	if location := resp.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}

	if properties := resp.DataLakeStoreAccountProperties; properties != nil {
		d.Set("tier", string(properties.CurrentTier))

		d.Set("encryption_state", string(properties.EncryptionState))
		d.Set("firewall_state", string(properties.FirewallState))
		d.Set("firewall_allow_azure_ips", string(properties.FirewallAllowAzureIps))

		if config := properties.EncryptionConfig; config != nil {
			d.Set("encryption_type", string(config.Type))
		}

		d.Set("endpoint", properties.Endpoint)
	}

	return tags.FlattenAndSet(d, resp.Tags)
}

func resourceArmDateLakeStoreDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Datalake.StoreAccountsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	name := id.Path["accounts"]
	future, err := client.Delete(ctx, resourceGroup, name)
	if err != nil {
		return fmt.Errorf("deleting Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for deletion of Data Lake Store %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	return nil
}
