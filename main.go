package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type regionData struct {
	id   string
	data []*ec2.ServiceDetail
}

func getData(region string, sess *session.Session, ch chan<- regionData) {
	fmt.Println("making request for " + region)
	svc := ec2.New(sess, aws.NewConfig().WithRegion(region))
	res, err := svc.DescribeVpcEndpointServices(&ec2.DescribeVpcEndpointServicesInput{})
	if err != nil {
		log.Fatal(err)
	}
	ch <- regionData{region, res.ServiceDetails}
}

func main() {
	// Create a EC2 client from just a session.
	sess := session.Must(session.NewSession())

	// Create a EC2 client with additional configuration
	svc := ec2.New(sess, aws.NewConfig().WithRegion("us-east-1"))

	// Keep track of all service types for headers
	var serviceList []string
	var regionList []string

	// Yo, I heard you like maps of strings...
	resultMap := make(map[string]map[string]bool)

	// First we make an API call to get all the regions, then we make an API call per region
	regions := getRegions(svc)

	ch := make(chan regionData)

	for _, region := range regions {
		regionList = append(regionList, *region.RegionName)
		go getData(*region.RegionName, sess, ch)
	}

	for range regions {
		res := (<-ch)

		resultMap[res.id] = make(map[string]bool)
		for _, item := range res.data {
			if *item.Owner == "amazon" {

				// Chop off the leading uri
				s := strings.Split(*item.ServiceName, ".")
				sname := s[len(s)-1]

				// Track the list of available services for pretty output
				serviceList = addService(serviceList, sname)
				resultMap[res.id][sname] = true
			}
		}
	}

	err := genCSV(serviceList, regionList, resultMap)
	if err != nil {
		log.Fatal(err)
	}
}

func addService(serviceList []string, newService string) []string {
	for _, oldService := range serviceList {
		if oldService == newService {
			return serviceList
		}
	}
	res := append(serviceList, newService)
	sort.Strings(res)
	return res
}

func getRegions(svc *ec2.EC2) []*ec2.Region {
	input := &ec2.DescribeRegionsInput{}
	result, err := svc.DescribeRegions(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
	}
	return result.Regions
}

func genCSV(serviceList []string, regionList []string, resultMap map[string]map[string]bool) error {
	file, err := os.Create("result.csv")
	if err != nil {
		return err
	}

	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Region"}
	for _, s := range serviceList {
		header = append(header, s)
	}
	err = writer.Write(header)
	if err != nil {
		return err
	}

	// Go randomizes map iteration, so we need to use a sorted index for order
	sort.Strings(regionList)

	for _, r := range regionList {
		row := []string{r}
		for _, s := range serviceList {
			_, ok := resultMap[r][s]
			if ok {
				row = append(row, "X")
			} else {
				row = append(row, "")
			}
		}
		writer.Write(row)
	}
	return nil
}
