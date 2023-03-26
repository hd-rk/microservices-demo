using System;
using System.Linq;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Caching.Distributed;
using MongoDB.Bson;
using MongoDB.Driver;
using Google.Protobuf;

namespace cartservice.cartstore
{
    public class MongoCartStore : ICartStore
    {
        private readonly IMongoCollection<Hipstershop.Cart> _cartCollection;

        public MongoCartStore(string connectionString, string databaseName)
        {
            var client = new MongoClient(connectionString);
            var database = client.GetDatabase(databaseName);
            _cartCollection = database.GetCollection<Hipstershop.Cart>("carts");
        }

        public async Task AddItemAsync(string userId, string productId, int quantity)
        {
            Console.WriteLine($"AddItemAsync called with userId={userId}, productId={productId}, quantity={quantity}");

            try
            {
                var filter = Builders<Hipstershop.Cart>.Filter.Eq("UserId", userId);
                var cart = await _cartCollection.Find(filter).FirstOrDefaultAsync();
                if (cart == null)
                {
                    cart = new Hipstershop.Cart();
                    cart.UserId = userId;
                    cart.Items.Add(new Hipstershop.CartItem { ProductId = productId, Quantity = quantity });
                    await _cartCollection.InsertOneAsync(cart);
                }
                else
                {
                    var existingItem = cart.Items.SingleOrDefault(i => i.ProductId == productId);
                    if (existingItem == null)
                    {
                        cart.Items.Add(new Hipstershop.CartItem { ProductId = productId, Quantity = quantity });
                    }
                    else
                    {
                        existingItem.Quantity += quantity;
                    }
                    await _cartCollection.ReplaceOneAsync(filter, cart);
                }
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
                var filter = Builders<Hipstershop.Cart>.Filter.Eq("UserId", userId);
                await _cartCollection.DeleteOneAsync(filter);
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
                var filter = Builders<Hipstershop.Cart>.Filter.Eq("UserId", userId);
                var cart = await _cartCollection.Find(filter).FirstOrDefaultAsync();
                if (cart == null)
                {
                    return new Hipstershop.Cart();
                }
                return cart;
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
                _cartCollection.Find(new BsonDocument()).FirstOrDefault();
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }
    }
}
