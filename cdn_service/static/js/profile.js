import CONTRACT_ABI from './MintNFTABI.js';
import { uploadToIPFS } from './IPFSService.js';
import { TICKET_CONTRACT_ADDRESS, connectMetaMask, getWeb3 } from './shared.js';

const AGGREGATOR_URL = '/api';

function profileApp() {
    return {
        loading: true,
        error: null,
        userId: null,
        orders: [],
        mintingOrderId: null,
        mintingStatus: '',

        init() {
            this.fetchProfile();
        },

        async fetchProfile() {
            try {
                this.loading = true;
                this.error = null;

                const token = localStorage.getItem('auth_token');
                const response = await fetch(`${AGGREGATOR_URL}/profile`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });

                if (response.status === 401) {
                    localStorage.removeItem('auth_token');
                    window.location.href = '/login';
                    return;
                }

                if (!response.ok) throw new Error(`HTTP ${response.status}`);

                const data = await response.json();
                this.userId = data.user_id;
                this.orders = data.orders || [];

                await this.checkMintedStatus();
            } catch (err) {
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },

        async checkMintedStatus() {
            if (typeof window.ethereum === 'undefined') return;

            try {
                const web3 = getWeb3();
                const contract = new web3.eth.Contract(CONTRACT_ABI, TICKET_CONTRACT_ADDRESS);

                await Promise.all(
                    this.orders
                        .filter(order => order.status === 'completed' && order.tickets?.length > 0)
                        .map(async (order) => {
                            order.minted = await contract.methods.ticketMinted(order.order_reference_id).call();
                        })
                );
            } catch (err) {
                console.error('Error checking minted status:', err);
            }
        },

        async mintOrder(order) {
            try {
                this.mintingOrderId = order.order_reference_id;
                this.mintingStatus = 'Connecting wallet...';

                const userAddress = await connectMetaMask();

                this.mintingStatus = 'Uploading metadata to IPFS...';

                const metadataList = order.tickets.map(ticket => ({
                    name: `${order.event.performer_name} - ${ticket.ticket_type.toUpperCase()} Ticket`,
                    description: `Seat ${ticket.seat_number} at ${order.event.venue_name}`,
                    image: order.event.display_image || '',
                    attributes: [
                        { trait_type: "Event", value: order.event.performer_name },
                        { trait_type: "Venue", value: order.event.venue_name },
                        { trait_type: "Seat Number", value: ticket.seat_number },
                        { trait_type: "Ticket Type", value: ticket.ticket_type },
                        { trait_type: "Original Order", value: order.order_reference_id },
                        { trait_type: "Ticket ID", value: ticket.id }
                    ]
                }));

                const ipfsUrls = await Promise.all(metadataList.map(uploadToIPFS));

                this.mintingStatus = 'Minting NFTs on blockchain...';

                const web3 = getWeb3();
                const contract = new web3.eth.Contract(CONTRACT_ABI, TICKET_CONTRACT_ADDRESS);

                const receipt = await contract.methods.mintBatch(
                    userAddress,
                    ipfsUrls,
                    order.order_reference_id
                ).send({ from: userAddress });

                const transferEvents = receipt.events.Transfer;
                const tokenIds = Array.isArray(transferEvents)
                    ? transferEvents.map(e => e.returnValues.tokenId)
                    : [transferEvents.returnValues.tokenId];

                alert(`🎉 Successfully minted ${tokenIds.length} NFT tickets!\n\nToken IDs: ${tokenIds.join(', ')}\n\nTransaction: ${receipt.transactionHash}`);

                await this.fetchProfile();
            } catch (error) {
                console.error('Minting failed:', error);
                alert('❌ Failed to mint tickets: ' + error.message);
            } finally {
                this.mintingOrderId = null;
                this.mintingStatus = '';
            }
        }
    }
}

function handleLogout() {
    localStorage.removeItem('auth_token');
    document.cookie = 'auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT';
    window.location.href = '/';
}

window.profileApp = profileApp;
window.handleLogout = handleLogout;