using System;
using System.Linq;
using System.Threading.Tasks;
using Grpc.Core;
using MongoDB.Bson;
using MongoDB.Driver;
using Google.Protobuf;

namespace cartservice.cartstore
{
    public class MongoCartStore : ICartStore
    {
        private readonly IMongoCollection<BsonDocument> _collection;

        public MongoCartStore(string mongoConnectionString, string mongoDatabaseName)
        {
            var client = new MongoClient(mongoConnectionString);
            var database = client.GetDatabase(mongoDatabaseName);
            _collection = database.GetCollection<BsonDocument>("carts");

            MongoDB.Bson.Serialization.BsonSerializer.RegisterSerializer(
                typeof(System.Dynamic.ExpandoObject),
                new MongoDB.Bson.Serialization.Serializers.ExpandoObjectSerializer()
            );
        }

        public async Task AddItemAsync(string userId, string productId, int quantity)
        {
            Console.WriteLine($"AddItemAsync called with userId={userId}, productId={productId}, quantity={quantity}");

            try
            {
                var filter = Builders<BsonDocument>.Filter.Eq("_id", userId);
                var update = Builders<BsonDocument>.Update.AddToSet("items", new BsonDocument
                {
                    { "product_id", productId },
                    { "quantity", quantity }
                });

                await _collection.UpdateOneAsync(filter, update, new UpdateOptions
                {
                    IsUpsert = true
                });
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.FailedPrecondition, $"Can't access cart storage. {ex}"));
            }
        }

        public async Task EmptyCartAsync(string userId)
        {
            Console.WriteLine($"EmptyCartAsync called with userId={userId}");

            try
            {
                var filter = Builders<BsonDocument>.Filter.Eq("_id", userId);
                var update = Builders<BsonDocument>.Update.Set("items", new BsonArray());

                await _collection.UpdateOneAsync(filter, update, new UpdateOptions
                {
                    IsUpsert = true
                });
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.FailedPrecondition, $"Can't access cart storage. {ex}"));
            }
        }

        public async Task<Hipstershop.Cart> GetCartAsync(string userId)
        {
            Console.WriteLine($"GetCartAsync called with userId={userId}");

            try
            {
                var filter = Builders<BsonDocument>.Filter.Eq("_id", userId);
                var document = await _collection.Find(filter).FirstOrDefaultAsync();

                if (document != null)
                {
                    var cart = new Hipstershop.Cart
                    {
                        UserId = document["_id"].AsString
                    };

                    foreach (BsonDocument item in document["items"].AsBsonArray)
                    {
                        cart.Items.Add(new Hipstershop.CartItem
                        {
                            ProductId = item["product_id"].AsString,
                            Quantity = item["quantity"].AsInt32
                        });
                    }

                    return cart;
                }

                return new Hipstershop.Cart();
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.FailedPrecondition, $"Can't access cart storage. {ex}"));
            }
        }

        public bool Ping()
        {
            try
            {
                var command = new BsonDocument { { "ping", 1 } };
                _collection.Database.RunCommand<BsonDocument>(command);
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }
    }
}
