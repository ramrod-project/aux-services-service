**Running the auxiliary container**

```
docker run -d --rm --name <name> --network <network> -p 80:80 -p 21:21 -p 20:20 -p 10090-10100:10090-10100 ramrodpcp/auxiliary-services:<tag>
```