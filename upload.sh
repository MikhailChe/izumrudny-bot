go vet ./... && \
go fmt ./... && 
yc serverless function version create \
  --function-id=d4eld3krf4lqpap8fe2p \
  --entrypoint index.Handler \
  --runtime golang119 \
  --memory '128MB' \
  --execution-timeout '5s' \
  --service-account-id ajes7c34cpugc3r8ms57 \
  --secret id=e6q537vrkm5h3k4bmt7g,key=token,environment-variable=TELEGRAM_TOKEN \
  --source-path . && \
yc serverless function set-scaling-policy \
  --id d4eld3krf4lqpap8fe2p \
  --tag \$latest \
  --zone-instances-limit=1 \
  --zone-requests-limit=2 \
  --provisioned-instances-count=0
