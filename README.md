## Reproduction Instructions for Cloud Function Blog Post

The following instructions will step you through the process of setting up the cloud provider environments and deploying code necessary to gather the information that lead to the above conclusions. In less than an hour you should be able to see the same results as well as have ongoing telemetry similar to have was used in the [Google's June 2nd Outage blog post](https://lightstep.com/blog/googles-june-2nd-outage-their-status-page-not-equal-to-reality/?utm_source=ls-research&amp;utm_medium=web&amp;utm_content=na&amp;utm_term=na&amp;adgroupid=na). Please share reproduction results or ask questions via&nbsp;[research@lightstep.com](mailto:research@lightstep.com) or [@LightStepLabs](https://twitter.com/LightStepLabs) on Twitter.

### Set Basic Environment Variables

The shell scripts below use GCP\_PROJECT to specify which Google Cloud Platform project to use for Cloud Storage, Functions, Cloud Scheduler, and AppEngine. They use BUCKET\_PREFIX to get unique bucket and function names. The associated GCP project will need billing enabled.

~~~bash
export GCP_PROJECT="<your project name here>"
export BUCKET_PREFIX="<your bucket prefix here>"
~~~

### Set Up Object Pool

The Cloud Function code pulls objects randomly from a pool to control for caching effects. In this version all objects will, on average, be pulled several times, but later research will show how this can change. Objects are each 1k of random data in the format below.

~~~
<pool size>_<object order>_<size>.obj
~~~

~~~bash
for ((i = 1; i < 11; i++)); do dd if=/dev/urandom of=10_${i}_1k.obj bs=1k count=1; done
~~~

### Google Cloud Storage Setup

#### IAM

Creating a service account allows us to give permissions to read from the cloud storage bucket automatically to the Cloud Function. The service account is named the same as our bucket prefix.

~~~bash
gcloud iam service-accounts create $BUCKET_PREFIX --project $GCP_PROJECT
~~~

#### Buckets

Create regional buckets in 4 regions allowing us to understand how performance varies for functions accessing objects in diverse locations. Because bucket names must be globally unique, the name of the bucket is the prefix and the region.

~~~bash
for bucket_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    gsutil mb -p $GCP_PROJECT -c regional -l $bucket_region -b on gs://$BUCKET_PREFIX-$bucket_region
done
~~~

For each of the buckets we need to give the service account object read permissions ("objectViewer").

~~~bash
for bucket_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    gsutil iam ch serviceAccount:$BUCKET_PREFIX@$GCP_PROJECT.iam.gserviceaccount.com:objectViewer gs://$BUCKET_PREFIX-$bucket_region
done
~~~

#### Objects

We then upload the 10 1k random files to each of the regional buckets.

~~~bash
for bucket_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    gsutil -m cp 10_*_1k.obj gs://$BUCKET_PREFIX-$bucket_region
done
~~~

### AWS S3 Setup

#### IAM

We create an IAM user for the Cloud Function to use to access S3 buckets. The user, the group, and the policy are all named with the bucket prefix. The below IAM policy give access to all regionally suffixed buckets and their contents. If you're running this in a production account, make sure that doesn't overlap with anything that it shouldn't.

~~~bash
aws iam create-user --user-name $BUCKET_PREFIX
aws iam create-access-key --user-name $BUCKET_PREFIX # Save key for function setup

aws iam create-group --group-name $BUCKET_PREFIX
aws iam add-user-to-group --group-name $BUCKET_PREFIX --user-name $BUCKET_PREFIX

aws iam create-policy --policy-name $BUCKET_PREFIX --policy-document '{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": "s3:ListBucket",
            "Resource": "arn:aws:s3:::'$BUCKET_PREFIX'-*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::'$BUCKET_PREFIX'-*/*"
        }
    ]
}'

aws iam attach-group-policy --group-name $BUCKET_PREFIX --policy-arn <arn from last step>
~~~

#### Buckets

We next create S3 buckets in 4 regions with prefix - region naming.

~~~bash
for bucket_region in "us-east-1" "us-east-2" "us-west-2" "eu-west-2";
do
    aws s3 mb s3://$BUCKET_PREFIX-$bucket_region --region $bucket_region
done
~~~

#### Objects

Then we upload the 10 1k random data files to all the buckets.

~~~bash
for bucket_region in "us-east-1" "us-east-2" "us-west-2" "eu-west-2";
do
    aws s3 cp . s3://$BUCKET_PREFIX-$bucket_region \
        --recursive \
        --exclude "*" \
        --include "10_*_1k.obj"
done
~~~

### LightStep Tracing Setup

The directions and analysis use LightStep \[*x*\]PM to collect the spans from the Cloud Function and analyze them with the Trace Analysis feature in Explorer. The LightStep tracer usage in the function can be replaced with any [OpenTracing](https://opentracing.io/)&nbsp;tracer and analysis done in other [OSS or proprietary systems](https://opentracing.io/docs/supported-tracers/). We welcome these reproductions of results as well.

You can sign up for a LightStep \[*x*\]PM Free Trial&nbsp;[here](https://go.lightstep.com/tracing.html?utm_source=ls-research&amp;utm_medium=web&amp;utm_content=na&amp;utm_term=na&amp;adgroupid=na). Then follow the link in email to complete signup.

Retrieve the Project Access Token from the Project Settings [page](https://docs.lightstep.com/docs/project-access-tokens) and paste into LS\_ACCESS\_TOKEN environment setting below.

### Google Cloud Functions Deployment

Next we deploy the Go application as a Cloud Function on GCP in the same 4 regions as the buckets. This may prompt you to enable Cloud Functions in the project to continue.

~~~bash
export LS_ACCESS_TOKEN="<access token from LightStep>"
export AWS_S3_ACCESS="<access key id from AWS S3 user>"
export AWS_S3_SECRET="<secret access key from AWS S3 user>"

for function_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    gcloud functions deploy --project $GCP_PROJECT ObjCheck \
        --runtime go111 \
        --trigger-http \
        --set-env-vars BUCKET_PREFIX=$BUCKET_PREFIX \
        --set-env-vars LS_ACCESS_TOKEN=$LS_ACCESS_TOKEN \
        --set-env-vars AWS_ACCESS_KEY_ID=$AWS_S3_ACCESS \
        --set-env-vars AWS_SECRET_ACCESS_KEY=$AWS_S3_SECRET \
        --set-env-vars GIT_TAG=$(git rev-parse --short HEAD) \
        --service-account $BUCKET_PREFIX@$GCP_PROJECT.iam.gserviceaccount.com \
        --region $function_region
done
~~~

### Google Cloud Scheduler Setup

The Google Cloud Function has a HTTP trigger. We use Cloud Scheduler entries for the complete set of function regions and bucket regions set to trigger every minute and cause the function to retrieve 50 random objects.

Running the below command may prompt you to enable Cloud Scheduler and App Engine for the project. The hosting region for App Engine should not affect the reproduction.

~~~bash
for function_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    for bucket_region in "us-central1" "us-east1" "asia-east2" "europe-west2" "us-east-1" "us-east-2" "us-west-2" "eu-west-2";
    do
        suffix=$(echo $bucket_region | cut -d'-' -f3)
        if [[ -n $suffix ]]; then service=s3; else service=gcs; fi
        gcloud beta scheduler jobs create http $BUCKET_PREFIX-$function_region-$bucket_region-10-10 \
            --schedule="* * * * *" \
            --uri=https://$function_region-$GCP_PROJECT.cloudfunctions.net/ObjCheck \
            --message-body='{"service": "'$service'", "region": "'$bucket_region'", "pool": 10, "count": 50}' \
            --project $GCP_PROJECT
    done
done
~~~

### LightStep Stream Setup

To track the performance of the full combination of functions and regional buckets, you need to set up Streams. We'll do this using that API. This is not necessary to reproduce the results but can be interesting for showing longer term trends. Under Project Settings, in the Identification box, find the Organization and Project values and paste into LS\_ORG and LS\_PROJECT below.

Create a LightStep API key with "Member" privileges using these [directions](https://docs.lightstep.com/docs/api-keys) and paste it below into LS\_API\_KEY.

~~~bash
export LS_ORG="<LightStep Org>"
export LS_PROJECT="<LightStep Project>"
export LS_API_KEY="<API Key (not Access Token)>

for function_region in "us-central1" "us-east1" "asia-east2" "europe-west2";
do
    for bucket_region in "us-central1" "us-east1" "asia-east2" "europe-west2" "us-east-1" "us-east-2" "us-west-2" "eu-west-2";
    do
        suffix=$(echo $bucket_region | cut -d'-' -f3)
        if [[ -n $suffix ]]; then service=s3; else service=gcs; fi
        curl --request POST \
            -H "Authorization: bearer $LS_API_KEY" \
            --url https://api.lightstep.com/public/v0.1/$LS_ORG/projects/$LS_PROJECT/searches \
            --data '{"data":{"attributes":{"name":"Requests from '$function_region' to '$bucket_region' ('$service')","query":"operation:\"requestObject\" tag:\"region\"=\"'$function_region'\" tag:\"bucket\"=\"'$BUCKET_PREFIX'-'$bucket_region'\""}, "type":"search"}}'
    done
done
~~~

### Analysis Process

After letting the functions run for about 10 minutes you should have sufficient data to analyze. Navigate to the Explorer tab on the left side of the interface.

To see first requests for GCS bucket resources add "operation: requestObject", "tag: seq="0"", and "tag: service="gcs"" to the query bar then click run to get a snapshot based on that query. You can then filter by region and group by bucket in the Trace Analysis to see p50 latency numbers from that region to the different buckets. Clicking on the different regions will show lists of traces. Clicking on traces will time spent in different parts of the request process.

Changing "tag: service="s3"" in the query bar and rerunning, the doing the same filtering and group by shows the different for first connections to S3. Clicking through to regional traces and then examine traces shows the different in latency for roundtrips.

Similarly you can change the sequence tag to "tag: seq="1"" to see second connections and compare GCS and S3 services to see how they become much more similar due to connection reuse.