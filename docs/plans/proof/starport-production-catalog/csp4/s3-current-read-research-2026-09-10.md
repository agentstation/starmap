# S3 current-read qualification research

This research selects no adapter and gives no production qualification credit.
The current S3 adapter leaves current-head receipt issuance disabled.

Amazon S3 documents strong consistency for object reads after successful writes and deletes.
Concurrent operations can overlap, so an observation must retain the receipt interval that started before the read.
The service guarantee applies to the bucket's object operations. [Amazon S3 consistency model](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html#ConsistencyModel)

The Go SDK permits caller-selected endpoints and endpoint resolvers.
A client type alone therefore does not identify the service that handles a request.
This is an inference from the SDK configuration contract. [AWS SDK endpoint configuration](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html)

Starmap must qualify the selected endpoint and transport before asserting the optional current-read capability.
Ordinary object reads and conditional writes remain separate capabilities.
No code change in this research extends the existing adapter's contract.
