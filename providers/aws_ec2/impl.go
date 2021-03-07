// Implements the 'aws_ec2' target type, which uses AWS SDK to create and
// terminate EC2 virtual machines.
package aws_ec2

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"golang.org/x/net/context"

	"github.com/stephank/lazyssh/providers"
)

func init() {
	providers.Register("aws_ec2", &Factory{})
}

type Factory struct{}

type Provider struct {
	BlockDeviceMappings []*types.BlockDeviceMapping
	AttachVolumes       []*ec2.AttachVolumeInput
	IamInstanceProfile  *types.IamInstanceProfileSpecification
	ImageId             string
	InstanceType        types.InstanceType
	KeyName             string
	Placement           *types.Placement
	SubnetId            *string
	UserData64          *string
	CheckPort           uint16
	Shared              bool
	Linger              time.Duration
	Ec2                 *ec2.Client
}

type state struct {
	id   string
	addr *string
}

type hclTarget struct {
	EbsBlockDevice     []*hclEbsBlockDevice `hcl:"ebs_block_device,block"`
	AttachVolumes      []*hclVolume         `hcl:"attach_volume,block"`
	Placement          *hclPlacement        `hcl:"placement,block"`
	ImageId            string               `hcl:"image_id,attr"`
	InstanceType       string               `hcl:"instance_type,attr"`
	KeyName            string               `hcl:"key_name,attr"`
	SubnetId           *string              `hcl:"subnet_id,optional"`
	UserData           *string              `hcl:"user_data,optional"`
	IamInstanceProfile string               `hcl:"iam_instance_profile,optional"`
	Profile            *string              `hcl:"profile,optional"`
	Region             *string              `hcl:"region,optional"`
	CheckPort          uint16               `hcl:"check_port,optional"`
	Shared             *bool                `hcl:"shared,optional"`
	Linger             string               `hcl:"linger,optional"`
}

type hclEbsBlockDevice struct {
	DeviceName          string  `hcl:"device_name,attr"`
	DeleteOnTermination *bool   `hcl:"delete_on_termination,optional"`
	Encrypted           *bool   `hcl:"encrypted,optional"`
	Iops                *int32  `hcl:"iops,optional"`
	KmsKeyId            *string `hcl:"kms_key_id,optional"`
	SnapshotId          *string `hcl:"snapshot_id,optional"`
	VolumeSize          *int32  `hcl:"volume_size,optional"`
	VolumeType          string  `hcl:"volume_type,optional"`
}

type hclVolume struct {
	DeviceName string `hcl:"device_name,attr"`
	VolumeId   string `hcl:"volume_id,optional"`
}

// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Placement.html
type hclPlacement struct {
	AvailabilityZone string `hcl:"availability_zone,optional"`
}

var errAttachVolume = errors.New("failed to attach volume")
var errUnknown = errors.New("something broke")

const requestTimeout = 30 * time.Second

func (factory *Factory) NewProvider(target string, hclBlock hcl.Body) (providers.Provider, error) {
	parsed := &hclTarget{}
	diags := gohcl.DecodeBody(hclBlock, nil, parsed)
	if diags.HasErrors() {
		return nil, diags
	}

	var cfgMods []config.Config
	if parsed.Profile != nil {
		cfgMods = append(cfgMods, config.WithSharedConfigProfile(*parsed.Profile))
	}
	if parsed.Region != nil {
		cfgMods = append(cfgMods, config.WithRegion(*parsed.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(cfgMods...)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Error loading AWS SDK configuration",
			Detail:   fmt.Sprintf("The AWS SDK reported an error while loading configuration: %s", err.Error()),
		})
	}

	prov := &Provider{
		Ec2:          ec2.NewFromConfig(awsCfg),
		ImageId:      parsed.ImageId,
		InstanceType: types.InstanceType(parsed.InstanceType),
		KeyName:      parsed.KeyName,
		SubnetId:     parsed.SubnetId,
	}

	if parsed.CheckPort == 0 {
		prov.CheckPort = 22
	} else {
		prov.CheckPort = parsed.CheckPort
	}

	if parsed.Shared == nil {
		prov.Shared = true
	} else {
		prov.Shared = *parsed.Shared
	}

	if prov.Shared {
		linger, err := time.ParseDuration(parsed.Linger)
		if err == nil {
			prov.Linger = linger
		} else {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid duration for 'linger' field",
				Detail:   fmt.Sprintf("The 'linger' value '%s' is not a valid duration: %s", parsed.Linger, err.Error()),
			})
		}
	} else if parsed.Linger != "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagWarning,
			Summary:  "Field 'linger' was ignored",
			Detail:   fmt.Sprintf("The 'linger' field has no effect for 'aws_ec2' targets with 'shared = false'"),
		})
	}

	for _, device := range parsed.EbsBlockDevice {
		prov.BlockDeviceMappings = append(prov.BlockDeviceMappings, &types.BlockDeviceMapping{
			DeviceName: aws.String(device.DeviceName),
			Ebs: &types.EbsBlockDevice{
				DeleteOnTermination: device.DeleteOnTermination,
				Encrypted:           device.Encrypted,
				Iops:                device.Iops,
				KmsKeyId:            device.KmsKeyId,
				SnapshotId:          device.SnapshotId,
				VolumeSize:          device.VolumeSize,
				VolumeType:          types.VolumeType(device.VolumeType),
			},
		})
	}

	for _, volume := range parsed.AttachVolumes {
		prov.AttachVolumes = append(prov.AttachVolumes, &ec2.AttachVolumeInput{
			Device:   aws.String(volume.DeviceName),
			VolumeId: aws.String(volume.VolumeId),
		})
	}

	prov.Placement = &types.Placement{}
	if parsed.Placement != nil {
		prov.Placement.AvailabilityZone = aws.String(parsed.Placement.AvailabilityZone)
	}

	if parsed.UserData != nil {
		prov.UserData64 = aws.String(base64.StdEncoding.EncodeToString([]byte(*parsed.UserData)))
	}

	if parsed.IamInstanceProfile != "" {
		prov.IamInstanceProfile = &types.IamInstanceProfileSpecification{
			Name: aws.String(parsed.IamInstanceProfile),
		}
	}

	if diags.HasErrors() {
		return nil, diags
	}

	return prov, diags
}

func (prov *Provider) IsShared() bool {
	return prov.Shared
}

func (prov *Provider) RunMachine(mach *providers.Machine) {
	ok, err := prov.start(mach)
	if errors.Is(err, errAttachVolume) {
		fmt.Printf("Error in Attaching Volumes. Stopping instance\n")
		prov.stop(mach)
	} else if err != nil {
		return
	}

	if ok {
		if prov.connectivityTest(mach) {
			prov.msgLoop(mach)
		}
		prov.stop(mach)
	}
}

func (prov *Provider) start(mach *providers.Machine) (bool, error) {
	bgCtx := context.Background()

	ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
	res, err := prov.Ec2.RunInstances(ctx, &ec2.RunInstancesInput{
		BlockDeviceMappings: prov.BlockDeviceMappings,
		MinCount:            aws.Int32(1),
		MaxCount:            aws.Int32(1),
		ImageId:             &prov.ImageId,
		InstanceType:        prov.InstanceType,
		KeyName:             &prov.KeyName,
		SubnetId:            prov.SubnetId,
		UserData:            prov.UserData64,
		IamInstanceProfile:  prov.IamInstanceProfile,
		Placement:           prov.Placement,
	})
	if err != nil {
		log.Printf("EC2 instance failed to start: %s\n", err.Error())
		return false, nil
	}

	inst := res.Instances[0]
	log.Printf("Created EC2 instance '%s'\n", *inst.InstanceId)

	for i := 0; i < 20 && inst.State.Name == "pending"; i++ {
		<-time.After(3 * time.Second)

		ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
		res, err := prov.Ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []*string{inst.InstanceId},
		})
		if err != nil {
			log.Printf("Could not check EC2 instance '%s' state: %s\n", *inst.InstanceId, err.Error())
			return false, nil
		}
		if res.Reservations == nil || res.Reservations[0].Instances == nil {
			log.Printf("EC2 instance '%s' disappeared while waiting for it to start\n", *inst.InstanceId)
			return false, nil
		}

		inst = res.Reservations[0].Instances[0]
	}

	if inst.State.Name != "running" {
		log.Printf("EC2 instance '%s' in unexpected state '%s'\n", *inst.InstanceId, inst.State.Name)
		return false, nil
	}

	log.Printf("EC2 instance '%s' is running\n", *inst.InstanceId)

	mach.State = &state{
		id:   *inst.InstanceId,
		addr: inst.PublicIpAddress,
	}

	// We're running, we can attach the volumes
	for _, v := range prov.AttachVolumes {
		v.InstanceId = inst.InstanceId
		_, err := prov.Ec2.AttachVolume(ctx, v)
		if err != nil {
			fmt.Printf("Error in attaching volume: %v\n", err)
			return false, fmt.Errorf("%w: %v", errAttachVolume, err)
		}
	}

	return true, nil
}

func (prov *Provider) stop(mach *providers.Machine) {
	state := mach.State.(*state)
	bgCtx := context.Background()
	ctx, _ := context.WithTimeout(bgCtx, requestTimeout)
	_, err := prov.Ec2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(state.id)},
	})
	if err != nil {
		log.Printf("EC2 instance '%s' failed to stop: %s\n", state.id, err.Error())
	}
	log.Printf("Terminated EC2 instance '%s'\n", state.id)
}

// Check port every 3 seconds for 2 minutes.
func (prov *Provider) connectivityTest(mach *providers.Machine) bool {
	state := mach.State.(*state)
	if state.addr == nil {
		log.Printf("EC2 instance '%s' does not have a public IP address\n", state.id)
		return false
	}
	checkAddr := fmt.Sprintf("%s:%d", *state.addr, prov.CheckPort)
	checkTimeout := 3 * time.Second
	var err error
	var conn net.Conn
	for i := 0; i < 40; i++ {
		checkStart := time.Now()
		conn, err = net.DialTimeout("tcp", checkAddr, checkTimeout)
		if err == nil {
			conn.Close()
			log.Printf("Connectivity test succeeded for EC2 instance '%s'\n", state.id)
			return true
		}
		time.Sleep(time.Until(checkStart.Add(checkTimeout)))
	}
	log.Printf("EC2 instance '%s' port check failed: %s\n", state.id, err.Error())
	return false
}

func (prov *Provider) msgLoop(mach *providers.Machine) {
	// TODO: Monitor machine status
	state := mach.State.(*state)
	active := <-mach.ModActive
	for active > 0 {
		for active > 0 {
			select {
			case mod := <-mach.ModActive:
				active += mod
			case msg := <-mach.Translate:
				msg.Reply <- fmt.Sprintf("%s:%d", *state.addr, msg.Port)
			case <-mach.Stop:
				return
			}
		}

		// Linger
		select {
		case mod := <-mach.ModActive:
			active += mod
		case <-time.After(prov.Linger):
			return
		}
	}
}
