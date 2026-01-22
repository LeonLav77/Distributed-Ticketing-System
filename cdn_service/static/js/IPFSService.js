const PINATA_JWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySW5mb3JtYXRpb24iOnsiaWQiOiJlZjE2YTc3Mi0yODZkLTRjOWItYjYzYi00ODE2OGYyM2VkZjIiLCJlbWFpbCI6ImxrYXJkYXNAc3R1ZGVudC51bmlwdS5ociIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJwaW5fcG9saWN5Ijp7InJlZ2lvbnMiOlt7ImRlc2lyZWRSZXBsaWNhdGlvbkNvdW50IjoxLCJpZCI6IkZSQTEifSx7ImRlc2lyZWRSZXBsaWNhdGlvbkNvdW50IjoxLCJpZCI6Ik5ZQzEifV0sInZlcnNpb24iOjF9LCJtZmFfZW5hYmxlZCI6ZmFsc2UsInN0YXR1cyI6IkFDVElWRSJ9LCJhdXRoZW50aWNhdGlvblR5cGUiOiJzY29wZWRLZXkiLCJzY29wZWRLZXlLZXkiOiJmMzNjM2ZlY2NkYWRhMmFiNWE1NSIsInNjb3BlZEtleVNlY3JldCI6ImNlYzZhNzBmMTA5YjlkZWJiZTY5NmRhNDk0MjU4YTM3MWNlMGFjNzA0M2U5OTZmYzYyZDkxOTViYzc4OGYxOGQiLCJleHAiOjE3OTk1MzkwMjh9.nPv_nqHiY3j6m3nIhCptLJtdTd3k1kK8DVdbgocS5Z0';

async function uploadToIPFS(metadata) {
    const response = await fetch('https://api.pinata.cloud/pinning/pinJSONToIPFS', {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${PINATA_JWT}`,
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            pinataContent: metadata,
            pinataMetadata: {
                name: `${metadata.name}.json`
            }
        })
    });

    if (!response.ok) {
        throw new Error(`Pinata API error: ${response.status}`);
    }

    const data = await response.json();
    const ipfsHash = data.IpfsHash;
    const ipfsUrl = `ipfs://${ipfsHash}`;
    
    return ipfsUrl;
}

export { uploadToIPFS };